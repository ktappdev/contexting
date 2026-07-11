# Feature: Symbol Indexing

## What

Index top-level code identifiers (functions, types, classes, constants) from each file and include them as searchable tokens alongside the existing filename/synonym scoring.

Currently, search only matches against:
- File and directory names
- LLM-generated synonyms for those names

With symbol indexing, search also matches against:
- Function names (`validateJWTToken`, `CreateUser`, `handlePayment`)
- Type/class names (`UserRepository`, `AuthMiddleware`)
- Constants and variables (`MAX_RETRIES`, `DefaultTimeout`)

### Example

You search: `"JWT validation"`

Without symbol indexing:
- Only matches if a file is literally called `jwt_validation.go` or has a synonym "JWT"
- Misses `http_client.go` which contains `func validateJWTToken()`

With symbol indexing:
- `http_client.go` scores a hit because `validateJWTToken` contains `jwt` and `validate`
- Agent gets the right file

---

## Why

The core problem this tool solves is helping agents find the right file without scanning the whole repo. Right now there's a gap: **the filename doesn't always reflect what's inside it**. This is extremely common in real codebases:

- `utils.go` contains `func ParseAuthHeader()`
- `client.go` contains `func RetryWithBackoff()`
- `middleware.go` contains `func RateLimitByIP()`

None of these are discoverable by filename alone. Synonyms help at the edges but can't cover every function name the LLM didn't predict.

Symbol indexing closes that gap **without extra LLM calls** — it's pure static analysis, fast, and free.

---

## How

### Parsing Strategy

The default extractor mode is `auto` (tree-sitter with regex fallback). You can override this with the `--symbol-extractor` flag on `ctxt init`:

- `auto` (default): Use tree-sitter for supported languages, fall back to regex for others
- `treesitter`: Force tree-sitter for all supported languages (fails for unsupported languages)
- `regex`: Use regex-based extraction for all languages

**Language support:**

| Language | Extensions | Parser |
|----------|-----------|--------|
| Go | `.go` | `go/parser` (stdlib) |
| Python | `.py` | tree-sitter (via gotreesitter, pure Go) |
| JavaScript | `.js`, `.mjs` | tree-sitter (via gotreesitter, pure Go) |
| TypeScript | `.ts`, `.tsx` | tree-sitter (via gotreesitter, pure Go) |
| Rust | `.rs` | tree-sitter (via gotreesitter, pure Go) |
| Svelte | `.svelte` | tree-sitter (via gotreesitter, pure Go) |
| Astro | `.astro` | tree-sitter (via gotreesitter, pure Go) |
| Vue | `.vue` | regex (fallback) |
| Ruby | `.rb` | regex (fallback) |

**Go files — use `go/parser` (stdlib)**

Go has a built-in AST parser. Zero dependencies, perfect accuracy, extracts exact top-level declarations:

```go
import (
    "go/parser"
    "go/token"
)

func extractGoSymbols(path string) ([]string, error) {
    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, path, nil, 0)
    if err != nil {
        return nil, err
    }
    var symbols []string
    for name := range f.Scope.Objects {
        symbols = append(symbols, name)
    }
    return symbols, nil
}
```

This gives you every top-level `func`, `type`, `var`, and `const` name — exactly what we want.

**Tree-sitter languages — Python, JS/TS, Rust, Svelte, Astro**

Tree-sitter (via gotreesitter, a pure Go implementation) provides accurate AST-based symbol extraction for these languages. It extracts top-level declarations:
- Python: `def`, `class`, `async def`
- JavaScript/TypeScript: `function`, `class`, `const`, `export function`, `export class`, `export const`, `interface`, `type`, `enum`
- Rust: `fn`, `pub fn`, `struct`, `enum`, `trait`, `impl`
- Svelte/Astro: script blocks with the above patterns

**Regex fallback — Vue, Ruby, and others**

For languages without tree-sitter support, regex line scanning extracts symbols. Read the file line by line and match declaration patterns:

| Language | Extensions | Patterns |
|----------|-----------|---------|
| Vue | `.vue` | `function `, `class `, `const `, `export function`, `export class`, `export const` |
| Ruby | `.rb` | `def `, `class `, `module ` |

Example regex for a line like `export function handlePayment(`:
```
^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)
```

Extract just the identifier name, not the full signature.

### Data Model Changes

Add a `Symbols` field to the `Node` struct:

```go
type Node struct {
    FullPath string  `json:"full_path"`
    Type     string  `json:"type"`
    Synonyms []string `json:"synonyms,omitempty"`
    Symbols  []string `json:"symbols,omitempty"`  // NEW
    Children []*Node  `json:"children,omitempty"`
}
```

Symbols are stored in `ctx_index.json` just like synonyms — no re-parsing on every search.

### Indexing Integration

In the indexer, after a file node is created:

1. Check if the file extension has a known parser
2. If yes, extract symbols and attach to the node
3. Store in `ctx_index.json`

On `watch`, when a file changes:
- Re-extract symbols for that file (fast, no LLM needed)
- Update the node in memory

### Search Scoring

Add symbol matching to `SearchHintsWithOptions` in `search.go`:

```go
for _, symbol := range node.Symbols {
    symLower := strings.ToLower(symbol)
    // tokenize the symbol name (split camelCase/snake_case)
    symTokens := tokenizeIdentifier(symLower)
    
    for _, token := range tokens {
        if symLower == token {
            score += 8   // exact symbol match
        } else if strings.Contains(symLower, token) {
            score += 5   // symbol contains query token
        }
        for _, symToken := range symTokens {
            if symToken == token {
                score += 4  // camelCase part matches
            }
        }
    }
}
```

**Scoring rationale:**
- Exact symbol match (+8): same weight as synonym exact match
- Symbol contains token (+5): slightly less than basename contains (+7)
- CamelCase part match (+4): partial credit for `validateJWT` → `jwt`

### CamelCase / snake_case Splitting

Identifier names need to be tokenized themselves. `validateJWTToken` should split into `["validate", "jwt", "token"]`:

```go
func tokenizeIdentifier(name string) []string {
    // split on underscores first
    // then split camelCase on transitions from lower→upper and upper→lower
    // return lowercase parts, filter short/common ones
}
```

This is what makes `"JWT validation"` find `validateJWTToken`.

---

## Implementation Status

**✓ Completed** — Symbol indexing is implemented and enabled by default.

- **Step 1 — Symbol extraction:** ✓ Implemented in `symbols.go` with `auto` mode (tree-sitter with regex fallback)
- **Step 2 — Data model:** ✓ `Symbols []string` added to `Node` in `node.go`
- **Step 3 — Indexing integration:** ✓ Called during tree build in `indexer.go` and on file changes in `index_manager.go`
- **Step 4 — Search scoring:** ✓ Symbol scoring added to `SearchHintsWithOptions` in `search.go`
- **Step 5 — Eval:** ✓ Eval cases for content-vs-name mismatch scenarios in `docs/bench_cases.json`

**Additional enhancements:**
- ✓ Tree-sitter support for Python, JS/TS, Rust, Svelte, Astro (via gotreesitter, pure Go)
- ✓ Import extraction for JS/TS files to improve LLM synonym context
- ✓ Path-suffix deduplication (`parentDir/basename` keys) to prevent duplicate filename collisions
- ✓ `--symbol-extractor` flag to choose between `auto`, `treesitter`, or `regex` modes

---

## What We Are Not Doing

- **No full-text search** — we only extract declaration lines, not file contents. Keeps it fast and the index small.
- **No type inference** — we don't resolve what types functions return or accept. Just names.
- **No cross-file reference tracking** — not building a call graph. That's a different tool.
- **No full-text search** — we only extract declaration lines, not file contents. Keeps it fast and the index small.
- **No type inference** — we don't resolve what types functions return or accept. Just names.
- **No cross-file reference tracking** — not building a call graph. That's a different tool.

---

## Import Extraction

For JavaScript and TypeScript files, tree-sitter also extracts ESM import statements. These imports are fed to the LLM during synonym generation to improve context awareness.

**Why this matters:** Generic filenames like `route.ts`, `handler.ts`, or `controller.ts` are common across codebases. Without import context, the LLM can only guess at the file's purpose. With import extraction, a `route.ts` file containing:

```typescript
import {clerkClient} from "@clerk/nextjs"
import {WebhookEvent} from "@clerk/nextjs/server"
```

Gets domain-accurate synonyms like "clerk webhook handler" instead of generic terms like "route handler".

**Implementation:** Imports are extracted during symbol extraction and included in the LLM prompt alongside the file's symbols (up to 10 symbols and imports total per file).

---

## Path-Suffix Deduplication

To prevent duplicate filenames from overwriting each other in the synonym map, ctxt uses `parentDir/basename` keys instead of bare basenames.

**Problem:** A large codebase might have 194 files named `route.ts` scattered across different directories. If synonyms were keyed only by basename (`route.ts`), each would overwrite the previous, leaving only one synonym set.

**Solution:** Synonym keys use the parent directory as a prefix: `webhook/route.ts`, `api/route.ts`, `auth/route.ts`. This ensures each file gets its own synonym entry while keeping the key readable and hierarchical.

**Example:**
- `webhook/route.ts` → `["clerk webhook handler", "webhook endpoint"]`
- `api/route.ts` → `["api route", "rest endpoint"]`
- `auth/route.ts` → `["auth handler", "login route"]`

---

## Expected Impact

- Agents searching for business logic by concept (`"payment processing"`, `"rate limiting"`, `"token refresh"`) will get dramatically better results
- Files with generic names (`utils.go`, `helpers.py`, `client.ts`) become discoverable
- No LLM cost increase
- Minimal performance impact: symbol extraction is I/O bound and happens at index time, not search time
