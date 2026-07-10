# MCP Setup

## Overview

Contexting's MCP (Model Context Protocol) server lets AI assistants like Claude Desktop and Cursor search your codebase using concept-based ranked search. Instead of hunting through files with grep or find, the AI can quickly locate which files handle specific concepts — "auth middleware", "payment handler", "JWT token refresh" — by querying a precomputed index of files, symbols, and LLM-generated synonyms.

The MCP server runs as a persistent process with an in-memory index and file watching. It exposes two tools: `search` for finding files and `status` for checking index health.

## How MCP Works

MCP is a standard protocol for AI assistants to use external tools. The AI client (Claude Desktop, Cursor) starts ctxt as a subprocess and communicates via stdin/stdout using JSON-RPC. Everything is local — no network calls, no API keys required for the search itself. The client auto-discovers available tools on startup, so you just talk normally and the AI decides when to call the search tool.

## Prerequisites

1. **Install ctxt:**
   ```bash
   make install
   ```
   This places the binary on your PATH.

2. **Build an index in your project:**
   ```bash
   cd your-project
   ctxt init .
   ```
   This creates `.ctxt/ctx_index.json` with files, symbols, and optional synonyms.

3. **(Optional) Set up LLM for synonyms:**
   ```bash
   export OPENROUTER_API_KEY="sk-or-v1-..."
   ```
   Synonyms improve conceptual search but aren't required.

## Claude Desktop Setup (macOS)

1. **Install ctxt** (if not already installed):
   ```bash
   make install
   ```

2. **Build an index** in your project directory:
   ```bash
   cd your-project
   ctxt init .
   ```

3. **Edit the Claude Desktop config file:**
   ```bash
   ~/Library/Application Support/Claude/claude_desktop_config.json
   ```

4. **Add ctxt to the `mcpServers` section:**
   ```json
   {
     "mcpServers": {
       "ctxt": {
         "command": "ctxt",
         "args": ["mcp"]
       }
     }
   }
   ```

5. **Restart Claude Desktop** to load the MCP server.

6. **Start chatting** — Claude can now search your codebase. Try asking:
   - "Find the auth middleware"
   - "Where is the payment handler?"
   - "Which file handles JWT token refresh?"

## Cursor Setup

Cursor supports MCP servers through its settings. Add the same server definition:

**Option 1: Project-specific config**
Create `.cursor/mcp.json` in your project root:
```json
{
  "mcpServers": {
    "ctxt": {
      "command": "ctxt",
      "args": ["mcp"]
    }
  }
}
```

**Option 2: Global settings**
Add the same `mcpServers` block in Cursor's MCP settings (Settings > MCP).

Restart Cursor to load the server.

## Available Tools

| Tool | Description |
|------|-------------|
| `search` | Search a codebase for files using concept-based ranked search. Faster and more relevant than grep or find for locating WHERE code lives. Query with plain keywords, concepts, partial filenames, or symbol names (e.g. "auth login", "jwt token refresh", "payment handler", "createUser"). Results are ranked by relevance score, not alphabetical. Use this instead of grep when you need to find which file handles a concept — grep finds what's INSIDE files, this finds WHICH files matter. Do not use for searching file contents (use grep) or file metadata like size/date/permissions (use find). |
| `status` | Check the health and coverage of the ctxt codebase index. Returns total indexed files, directories, index generation time, and root path. Use this to verify the index is built and current before searching, or to diagnose why search results may be empty or stale. |

## Usage Examples

**Example 1: Find by concept**
```
You: Where is the authentication middleware?
Claude: [calls search tool with "authentication middleware"]
Claude: The authentication middleware is in internal/auth/middleware.go
```

**Example 2: Find by function name**
```
You: Which file contains the createUser function?
Claude: [calls search tool with "createUser"]
Claude: createUser is defined in internal/user/service.go
```

**Example 3: Find by partial filename**
```
You: Show me the config loader
Claude: [calls search tool with "config loader"]
Claude: The config loader is in config.go
```

**Example 4: Check index health**
```
You: Is the index up to date?
Claude: [calls status tool]
Claude: The index covers 234 files across 45 directories, generated 2 hours ago from /path/to/project
```

## Optional Flags

The MCP server accepts optional flags in the `args` array:

| Flag | Description | Default |
|------|-------------|---------|
| `--http` | Also serve HTTP search endpoint alongside MCP | disabled |
| `--llm-on-watch` | Enable live LLM synonym generation for new files | false |
| `--verbose` / `-v` | Enable verbose logging to stderr | disabled |
| `--debounce` | Filesystem event coalesce interval | 750ms |

**Example with flags:**
```json
{
  "mcpServers": {
    "ctxt": {
      "command": "ctxt",
      "args": ["mcp", "--verbose", "--llm-on-watch"]
    }
  }
}
```

## Troubleshooting

**Binary not on PATH**
- Claude/Cursor can't find the `ctxt` command
- Run `make install` to ensure it's on your PATH
- Verify with `which ctxt`

**No index found**
- The MCP server reports no index available
- Run `ctxt init .` in your project directory
- Ensure the MCP server is running from the correct working directory (some clients set this, others don't)

**Stale search results**
- Files you just created aren't appearing in search
- The MCP server watches for changes automatically
- If changes aren't picked up, restart the AI client to reload the MCP server

**LLM synonyms not working**
- Search works but conceptual queries are weak
- Set `OPENROUTER_API_KEY` environment variable
- Or add `api_key` to `.ctxt/ctx_config.toml`
- Restart the AI client after setting the key

**Permission errors**
- The MCP server can't read files or write the index
- Ensure ctxt has read access to your project directory
- Check that `.ctxt/` directory is writable

## Related Commands

- `ctxt init` — Build the index before using MCP
- `ctxt doctor` — Diagnose index and config issues
- `ctxt status` — Check index health from the command line
- `ctxt search-hints` — Test search queries manually
