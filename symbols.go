package main

import (
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// extractSymbols dispatches to the appropriate extractor based on file extension
func extractSymbols(path string) ([]string, error) {
	ext := strings.ToLower(path)
	if strings.HasSuffix(ext, ".go") {
		return extractGoSymbols(path)
	}
	if strings.HasSuffix(ext, ".py") {
		return extractByRegex(path, []string{
			`(?m)^(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)`,
		})
	}
	if strings.HasSuffix(ext, ".js") || strings.HasSuffix(ext, ".mjs") || strings.HasSuffix(ext, ".jsx") {
		return extractByRegex(path, []string{
			`(?m)^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^(?:export\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^(?:export\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`,
		})
	}
	if strings.HasSuffix(ext, ".ts") || strings.HasSuffix(ext, ".tsx") {
		return extractByRegex(path, []string{
			`(?m)^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^(?:export\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^(?:export\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`,
			`(?m)^(?:export\s+)?interface\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^(?:export\s+)?type\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^(?:export\s+)?enum\s+([A-Za-z_][A-Za-z0-9_]*)`,
		})
	}
	if strings.HasSuffix(ext, ".rs") {
		return extractByRegex(path, []string{
			`(?m)^(?:pub\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^struct\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^enum\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^trait\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^impl\s+(?:[^<]*<)?([A-Za-z_][A-Za-z0-9_]*)`,
		})
	}
	if strings.HasSuffix(ext, ".rb") {
		return extractByRegex(path, []string{
			`(?m)^def\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)`,
			`(?m)^module\s+([A-Za-z_][A-Za-z0-9_]*)`,
		})
	}
	if strings.HasSuffix(ext, ".vue") {
		return extractVueSymbols(path)
	}
	if strings.HasSuffix(ext, ".svelte") {
		return extractSvelteSymbols(path)
	}
	if strings.HasSuffix(ext, ".astro") {
		return extractAstroSymbols(path)
	}
	return nil, nil
}

// extractGoSymbols uses go/parser to extract top-level declarations
func extractGoSymbols(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		return nil, err
	}
	var symbols []string
	for name := range f.Scope.Objects {
		symbols = append(symbols, name)
	}
	return dedupeStrings(symbols), nil
}

// extractByRegex reads a file and extracts symbols matching the given patterns
func extractByRegex(path string, patterns []string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return extractByRegexContent(string(content), patterns)
}

// extractByRegexContent extracts symbols from content matching the given patterns
func extractByRegexContent(content string, patterns []string) ([]string, error) {
	var symbols []string
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbols = append(symbols, match[1])
			}
		}
	}
	return dedupeStrings(symbols), nil
}

// extractScriptBlock extracts all content between opening and closing patterns.
// Returns the concatenated inner content of all matching blocks, the full opening tag text
// of the first block (or empty string), and whether any block was found.
func extractScriptBlock(content string, openPattern, closePattern string) (string, string, bool) {
	openRe := regexp.MustCompile(openPattern)
	closeRe := regexp.MustCompile(closePattern)

	var found bool
	var firstOpeningTag string
	var builder strings.Builder

	offset := 0
	for {
		openMatch := openRe.FindStringIndex(content[offset:])
		if openMatch == nil {
			break
		}
		found = true

		// Capture the full opening tag text for the first block
		if firstOpeningTag == "" {
			firstOpeningTag = content[offset+openMatch[0] : offset+openMatch[1]]
		}

		// Search for close pattern after the open match
		contentAfterOpen := content[offset+openMatch[1]:]
		closeMatch := closeRe.FindStringIndex(contentAfterOpen)
		if closeMatch == nil {
			break
		}

		// Extract content between the end of open match and start of close match
		blockContent := contentAfterOpen[:closeMatch[0]]
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(blockContent)

		// Advance offset past the close tag
		// closeMatch indices are relative to contentAfterOpen
		closeEnd := closeMatch[1]
		offset = offset + openMatch[1] + closeEnd
	}

	return builder.String(), firstOpeningTag, found
}

var jsPatterns = []string{
	`(?m)^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^(?:export\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^(?:export\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`,
}

var tsPatterns = []string{
	`(?m)^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^(?:export\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^(?:export\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`,
	`(?m)^(?:export\s+)?interface\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^(?:export\s+)?type\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^(?:export\s+)?enum\s+([A-Za-z_][A-Za-z0-9_]*)`,
}

var jsPatternsWhitespace = []string{
	`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^\s*(?:export\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^\s*(?:export\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`,
}

var tsPatternsWhitespace = []string{
	`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^\s*(?:export\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^\s*(?:export\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`,
	`(?m)^\s*(?:export\s+)?interface\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^\s*(?:export\s+)?type\s+([A-Za-z_][A-Za-z0-9_]*)`,
	`(?m)^\s*(?:export\s+)?enum\s+([A-Za-z_][A-Za-z0-9_]*)`,
}

// extractVueSymbols extracts symbols from Vue single-file components
func extractVueSymbols(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Extract all script blocks
	scriptContent, openingTag, found := extractScriptBlock(string(content), `<script[^>]*>`, `</script>`)
	if !found {
		return nil, nil
	}

	// Check if any script uses TypeScript (lang attribute is in the opening tag)
	isTS := strings.Contains(openingTag, "lang=\"ts\"") || strings.Contains(openingTag, "lang='ts'")

	if isTS {
		return extractByRegexContent(scriptContent, tsPatternsWhitespace)
	}
	return extractByRegexContent(scriptContent, jsPatternsWhitespace)
}

// extractSvelteSymbols extracts symbols from Svelte single-file components
func extractSvelteSymbols(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Extract all script blocks
	scriptContent, openingTag, found := extractScriptBlock(string(content), `<script[^>]*>`, `</script>`)
	if !found {
		return nil, nil
	}

	// Check if any script uses TypeScript (lang attribute is in the opening tag)
	isTS := strings.Contains(openingTag, "lang=\"ts\"") || strings.Contains(openingTag, "lang='ts'")

	if isTS {
		return extractByRegexContent(scriptContent, tsPatternsWhitespace)
	}
	return extractByRegexContent(scriptContent, jsPatternsWhitespace)
}

// extractAstroSymbols extracts symbols from Astro frontmatter
func extractAstroSymbols(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	contentStr := string(content)

	// Find frontmatter delimiters using line-anchored regex
	dashRe := regexp.MustCompile(`(?m)^---\s*$`)
	matches := dashRe.FindAllStringIndex(contentStr, -1)
	if len(matches) < 2 {
		return nil, nil
	}

	// Ensure the opening --- is at or near the top of the file
	firstMatchStart := matches[0][0]
	if firstMatchStart > 0 {
		before := strings.TrimSpace(contentStr[:firstMatchStart])
		if before != "" {
			return nil, nil
		}
	}

	// Extract frontmatter content between the first two --- delimiters
	start := matches[0][1]
	end := matches[1][0]
	frontmatter := strings.TrimSpace(contentStr[start:end])

	// Use TypeScript patterns (TS is a superset of JS)
	return extractByRegexContent(frontmatter, tsPatternsWhitespace)
}

// tokenizeIdentifier splits CamelCase and snake_case into component tokens
func tokenizeIdentifier(name string) []string {
	var tokens []string

	// First split on underscores
	parts := strings.Split(name, "_")

	for _, part := range parts {
		if part == "" {
			continue
		}
		// Split on camelCase boundaries including acronyms
		// Strategy: track runs of uppercase, split when we see upper→lower with multiple uppers
		var current []rune
		for i, r := range part {
			if i > 0 {
				prev := rune(part[i-1])
				// lower→upper: start new word (camelCase like createUser)
				if unicode.IsLower(prev) && unicode.IsUpper(r) {
					if len(current) > 0 {
						tokens = append(tokens, string(current))
					}
					current = []rune{r}
				// upper→lower after uppercase sequence: end of acronym (like JWT in validateJWTToken)
				// e.g., "JWTToken": at 'o', we split "JWT" and start "Token"
				} else if unicode.IsUpper(prev) && unicode.IsLower(r) && len(current) > 1 {
					// current has multiple uppers like "JWT", split off all but last
					// then start new word with (last upper + this lower)
					lastUpper := current[len(current)-1]
					tokens = append(tokens, string(current[:len(current)-1]))
					current = []rune{lastUpper, r}
				} else if unicode.IsDigit(prev) && !unicode.IsDigit(r) {
					// digit→letter
					if len(current) > 0 {
						tokens = append(tokens, string(current))
					}
					current = []rune{r}
				} else if !unicode.IsDigit(prev) && unicode.IsDigit(r) {
					// letter→digit
					if len(current) > 0 {
						tokens = append(tokens, string(current))
					}
					current = []rune{r}
				} else {
					current = append(current, r)
				}
			} else {
				current = append(current, r)
			}
		}
		if len(current) > 0 {
			tokens = append(tokens, string(current))
		}
	}

	// Lowercase everything and dedupe
	var lower []string
	seen := make(map[string]struct{})
	for _, t := range tokens {
		low := strings.ToLower(t)
		if low == "" || len(low) < 2 {
			continue
		}
		if _, ok := seen[low]; !ok {
			seen[low] = struct{}{}
			lower = append(lower, low)
		}
	}
	return lower
}
