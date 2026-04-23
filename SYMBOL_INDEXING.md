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

Two approaches depending on file type:

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

**All other languages — regex line scanning**

Read the file line by line. Match lines that start with known declaration patterns. Extract the identifier that follows.

Language patterns:

| Language | Extensions | Patterns |
|----------|-----------|---------|
| Python | `.py` | `def `, `class `, `async def ` |
| JavaScript | `.js`, `.mjs` | `function `, `class `, `const `, `export function`, `export class`, `export const` |
| TypeScript | `.ts`, `.tsx` | same as JS + `interface `, `type `, `enum ` |
| Rust | `.rs` | `fn `, `pub fn `, `struct `, `enum `, `trait `, `impl ` |
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

Symbols are stored in `context.json` just like synonyms — no re-parsing on every search.

### Indexing Integration

In the indexer, after a file node is created:

1. Check if the file extension has a known parser
2. If yes, extract symbols and attach to the node
3. Store in `context.json`

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

## Implementation Plan

### Step 1 — Symbol extraction
- Create `symbols.go`
- Implement `extractSymbols(path string) []string` dispatcher
- Implement `extractGoSymbols` using `go/parser`
- Implement `extractByRegex` for Python, JS/TS, Rust
- Write unit tests with sample file fixtures

### Step 2 — Data model
- Add `Symbols []string` to `Node` in `indexer.go`
- Backward compatible: old `context.json` files without `symbols` still load fine (field is omitempty)

### Step 3 — Indexing integration
- Call `extractSymbols` during tree build in `indexer.go`
- Call it again on file change events in `index_manager.go`

### Step 4 — Search scoring
- Add symbol scoring block to `SearchHintsWithOptions` in `search.go`
- Add `tokenizeIdentifier` helper
- Update `SearchResult.Matches` to show `sym:validateJWTToken` for transparency

### Step 5 — Eval
- Add eval cases that specifically test content-vs-name mismatch scenarios
- E.g. query `"retry logic"` should hit `http_client.go` which has `RetryWithBackoff`
- Measure Hit@1/3/5 improvement vs baseline

---

## What We Are Not Doing

- **No full-text search** — we only extract declaration lines, not file contents. Keeps it fast and the index small.
- **No type inference** — we don't resolve what types functions return or accept. Just names.
- **No cross-file reference tracking** — not building a call graph. That's a different tool.
- **No tree-sitter dependency** — regex + stdlib is good enough and keeps the binary simple.

---

## Expected Impact

- Agents searching for business logic by concept (`"payment processing"`, `"rate limiting"`, `"token refresh"`) will get dramatically better results
- Files with generic names (`utils.go`, `helpers.py`, `client.ts`) become discoverable
- No LLM cost increase
- Minimal performance impact: symbol extraction is I/O bound and happens at index time, not search time
