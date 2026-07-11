package contexting

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// SymbolsExtractorMode controls which extraction engine is used.
// "auto":       tree-sitter first, fall back to regex on any error.
// "treesitter": tree-sitter only, propagate errors.
// "regex":      current behavior, regex only.
// Default is "auto".
var SymbolsExtractorMode = "auto"

// Query strings empirically validated against gotreesitter v0.23.1.
// Keep these aligned with the spike in /tmp/ts-spike.
const (
	pythonQuery = `
		(function_definition name: (identifier) @name)
		(class_definition name: (identifier) @name)
	`

	javascriptQuery = `
		(function_declaration name: (identifier) @name)
		(class_declaration name: (identifier) @name)
		(method_definition name: (property_identifier) @name)
		(variable_declarator name: (identifier) @name value: (arrow_function))
	`

	typescriptQuery = `
		(function_declaration name: (identifier) @name)
		(method_definition name: (property_identifier) @name)
		(class_declaration name: (type_identifier) @name)
		(interface_declaration name: (type_identifier) @name)
		(type_alias_declaration name: (type_identifier) @name)
		(enum_declaration name: (identifier) @name)
		(variable_declarator name: (identifier) @name value: (arrow_function))
	`

	rustQuery = `
		(function_item name: (identifier) @name)
		(struct_item name: (type_identifier) @name)
		(enum_item name: (type_identifier) @name)
		(trait_item name: (type_identifier) @name)
		(impl_item type: (type_identifier) @name)
	`

	// importQuery captures the string_fragment (no quotes) from every
	// import_statement's `source` field. Works for ESM imports like
	//   import { x } from "@clerk/nextjs"
	// and `import type` lines.
	// Empirically validated against gotreesitter v0.23.1's JS and TS grammars.
	importQuery = `
		(import_statement source: (string (string_fragment) @import))
	`
)

// extractSymbolsTreeSitter extracts symbols using tree-sitter grammars.
// Dispatches to a per-language extractor based on file extension.
// Returns an error for unsupported extensions or parse failures so the
// caller (the dispatcher in symbols.go) can fall back to regex.
func extractSymbolsTreeSitter(path string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".py":
		return extractWithQueryForLanguage(path, grammars.PythonLanguage(), pythonQuery)
	case ".js", ".mjs", ".jsx":
		return extractWithQueryForLanguage(path, grammars.JavascriptLanguage(), javascriptQuery)
	case ".ts", ".tsx":
		return extractWithQueryForLanguage(path, grammars.TypescriptLanguage(), typescriptQuery)
	case ".rs":
		return extractWithQueryForLanguage(path, grammars.RustLanguage(), rustQuery)
	case ".svelte":
		return extractSvelteSymbolsTreeSitter(path)
	case ".astro":
		return extractAstroSymbolsTreeSitter(path)
	default:
		return nil, fmt.Errorf("unsupported extension for tree-sitter: %s", ext)
	}
}

// extractWithQueryForLanguage is a convenience that reads the file and runs the query.
func extractWithQueryForLanguage(path string, lg *ts.Language, queryStr string) ([]string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return extractWithQuery(src, lg, queryStr)
}

// extractWithQuery parses src with the given language and runs the query,
// collecting text from any capture bound to @name. The result is deduplicated
// but otherwise returned in match order.
func extractWithQuery(src []byte, lg *ts.Language, queryStr string) ([]string, error) {
	parser := ts.NewParser(lg)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	root := tree.RootNode()

	q, err := ts.NewQuery(queryStr, lg)
	if err != nil {
		return nil, fmt.Errorf("compile query: %w", err)
	}

	cursor := q.Exec(root, lg, src)
	seen := make(map[string]struct{})
	var symbols []string

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, cap := range match.Captures {
			name := cap.Node.Text(src)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			symbols = append(symbols, name)
		}
	}

	return symbols, nil
}

// extractSvelteSymbolsTreeSitter extracts the inner contents of <script> blocks
// from a Svelte file, then parses them with the JavaScript or TypeScript
// tree-sitter grammar (chosen by lang="ts" on the opening tag).
//
// We don't query the Svelte grammar directly because we want per-declaration
// names, and Svelte's grammar is HTML-shaped — extracting <script> contents
// and reusing the JS/TS queries gives consistent results with the regex path.
func extractSvelteSymbolsTreeSitter(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	scriptContent, openingTag, found := extractScriptBlock(string(content), `<script[^>]*>`, `</script>`)
	if !found {
		return nil, nil
	}

	isTS := strings.Contains(openingTag, "lang=\"ts\"") || strings.Contains(openingTag, "lang='ts'")
	if isTS {
		return extractWithQuery([]byte(scriptContent), grammars.TypescriptLanguage(), typescriptQuery)
	}
	return extractWithQuery([]byte(scriptContent), grammars.JavascriptLanguage(), javascriptQuery)
}

// extractAstroSymbolsTreeSitter extracts the frontmatter (between the leading
// --- delimiters at the top of the file) and parses it with the TypeScript
// grammar. Astro frontmatter is always TS-flavored at the syntax level.
func extractAstroSymbolsTreeSitter(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	frontmatter, found := extractAstroFrontmatter(string(content))
	if !found {
		return nil, nil
	}

	return extractWithQuery([]byte(frontmatter), grammars.TypescriptLanguage(), typescriptQuery)
}

// extractFileImports reads a JS/TS/Svelte source file and returns its
// ESM import paths (e.g. "@clerk/nextjs", "next/server", "./db") in
// the order they appear, deduplicated.
//
// Supports: .ts, .tsx, .js, .jsx, .mjs, .svelte
// Returns nil for other extensions, for files that don't exist, or
// for parse failures. Callers should treat nil as "no signal" — it
// is never an error.
func extractFileImports(path string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".ts", ".tsx":
		return extractImportsFromSource(src, grammars.TypescriptLanguage())
	case ".js", ".jsx", ".mjs":
		return extractImportsFromSource(src, grammars.JavascriptLanguage())
	case ".svelte":
		// Svelte: extract <script> blocks first, then parse with JS or TS
		// based on the `lang` attribute (mirrors extractSvelteSymbolsTreeSitter).
		scriptContent, openingTag, found := extractScriptBlock(string(src), `<script[^>]*>`, `</script>`)
		if !found {
			return nil
		}
		isTS := strings.Contains(openingTag, "lang=\"ts\"") || strings.Contains(openingTag, "lang='ts'")
		if isTS {
			return extractImportsFromSource([]byte(scriptContent), grammars.TypescriptLanguage())
		}
		return extractImportsFromSource([]byte(scriptContent), grammars.JavascriptLanguage())
	default:
		return nil
	}
}

// extractImportsFromSource parses src with the given language and returns
// the import paths captured by importQuery. Returns nil on any failure
// (parse error, query compile error, no imports). Output is deduplicated
// but otherwise in match order.
func extractImportsFromSource(src []byte, lg *ts.Language) []string {
	parser := ts.NewParser(lg)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil
	}
	q, err := ts.NewQuery(importQuery, lg)
	if err != nil {
		return nil
	}

	cursor := q.Exec(tree.RootNode(), lg, src)
	seen := make(map[string]struct{})
	var imports []string
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, cap := range match.Captures {
			name := cap.Node.Text(src)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			imports = append(imports, name)
		}
	}
	return imports
}

// extractAstroFrontmatter returns the content between the first two --- lines
// at the top of an Astro file, and whether such a block was found.
func extractAstroFrontmatter(content string) (string, bool) {
	dashRe := regexp.MustCompile(`(?m)^---\s*$`)
	matches := dashRe.FindAllStringIndex(content, -1)
	if len(matches) < 2 {
		return "", false
	}

	// The opening --- must be at the very top of the file (only whitespace before it).
	firstStart := matches[0][0]
	if firstStart > 0 {
		before := strings.TrimSpace(content[:firstStart])
		if before != "" {
			return "", false
		}
	}

	start := matches[0][1]
	end := matches[1][0]
	return strings.TrimSpace(content[start:end]), true
}
