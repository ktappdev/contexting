# MCP Setup

## Overview

Contexting's MCP (Model Context Protocol) server lets AI assistants like Claude Desktop and Cursor search your codebase using concept-based ranked search. Instead of hunting through files with grep or find, the AI can quickly locate which files handle specific concepts — "auth middleware", "payment handler", "JWT token refresh" — by querying a precomputed index of files, symbols, and LLM-generated synonyms.

The MCP server runs as a persistent process with an in-memory index and file watching. It exposes two tools: `search` for finding files and `status` for checking index health. Both tools are read-only.

**Working directory:** if you omit the project path (`ctxt mcp`), the server uses the working directory the client launches it from. Pass the project's absolute path (e.g. `ctxt mcp /path/to/project`) to make setup independent of how the client is started.

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

## One-command setup

After building the index, run this from the project root:

```bash
ctxt mcp setup .
```

`setup` detects installed Claude Code, Codex, and Pi clients, shows the
project-local config files it will create or update, and asks before writing.
It then starts the MCP server in the foreground; stop it with Ctrl+C. The
directly launched stdio process does not attach to an already-open client. Stop
it with Ctrl+C, then restart or reload the clients; each client launches its
own configured server process.

To configure selected clients explicitly, use repeatable `--client` flags:

```bash
ctxt mcp setup --client codex --client pi .
```

For automation, `--yes` accepts the displayed config changes without a prompt:

```bash
ctxt mcp setup --yes .
```

The setup command writes only the MCP entry for `ctxt` and preserves other
servers in those files. It uses `.mcp.json` for Claude Code,
`.codex/config.toml` for Codex, and `.pi/mcp.json` for Pi. `ctxt mcp` uses local
stdio, so this setup is for clients running on the same machine; cloud-hosted
agents need a remotely reachable MCP transport.

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

4. **Add ctxt to the `mcpServers` section. Pass your project's absolute path** so the server targets the right project regardless of the working directory the client launches it from:
   ```json
   {
     "mcpServers": {
       "ctxt": {
         "command": "ctxt",
         "args": ["mcp", "/absolute/path/to/your-project"]
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
      "args": ["mcp", "/absolute/path/to/your-project"]
    }
  }
}
```

**Option 2: Global settings**
Add the same `mcpServers` block in Cursor's MCP settings (Settings > MCP), passing the project's absolute path so it works no matter which directory Cursor launches servers from.

Restart Cursor to load the server.

## Claude Code Setup

Claude Code discovers MCP servers from a project-level `.mcp.json`, or via `claude mcp add`:

```bash
claude mcp add ctxt -- ctxt mcp /absolute/path/to/your-project
```

Or create `.mcp.json` in your project root:
```json
{
  "mcpServers": {
    "ctxt": {
      "command": "ctxt",
      "args": ["mcp", "/absolute/path/to/your-project"]
    }
  }
}
```

Passing the project's absolute path is important for Claude Code: it is often launched from a directory that is not the project root, and the server resolves the config and index from the path you give it.

## Available Tools

| Tool | Description |
|------|-------------|
| `search` | Search a codebase for files using concept-based ranked search. Faster and more relevant than grep or find for locating WHERE code lives. Query with plain keywords, concepts, partial filenames, or symbol names (e.g. "auth login", "jwt token refresh", "payment handler", "createUser"). Results are ranked by relevance score (exact filenames and symbol hits rank highest; synonyms and partial path/basename matches add smaller weights). The server keeps the index current automatically — there is no separate watch or init step while it runs. Use this instead of grep when you need to find which file handles a concept — grep finds what's INSIDE files, this finds WHICH files matter. Do not use for searching file contents (use grep) or file metadata like size/date/permissions (use find). Both tools are read-only and never modify your project. |
| `status` | Report the health and coverage of the ctxt codebase index. Returns total indexed files, directories, index generation time, and root path. Use this to check what the live index covers before searching. |

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
| `--symbol-extractor` | Symbol extraction mode: auto, treesitter, regex | auto |

**Example with flags:**
```json
{
  "mcpServers": {
    "ctxt": {
      "command": "ctxt",
      "args": ["mcp", "--verbose", "--llm-on-watch", "--symbol-extractor", "treesitter"]
    }
  }
}

### Search Tool Options

The `search` tool's input schema is bounded so agents pass predictable values. All fields are optional except `query`:

| Option | Type | Description | Constraint |
|--------|------|-------------|------------|
| `query` | string | Keywords, filenames, or concepts | required |
| `limit` | integer | Maximum number of results | 1–50 (default 10) |
| `type` | string | Filter results to files or directories | `all`, `files`, or `dirs` only |
| `explain` | boolean | Include per-result score breakdown | — |
| `hybrid` | boolean | Augment index results with content matching via ripgrep | — |

## Troubleshooting

**Binary not on PATH**
- Claude/Cursor can't find the `ctxt` command
- Run `make install` to ensure it's on your PATH
- Verify with `which ctxt`

**Server fails to start: no index**
- The MCP server won't start without an existing index
- Run `ctxt init .` in the project directory first
- Configure the server with the project's absolute path so index and config are found no matter what working directory the client launches from

**Server fails to start: config**
- A missing config file is no longer an error — ctxt falls back to defaults — so the previous "config file not found" failure has been removed. Only the index is required.

**Multiple agents / read-only mode**
- Only the first `ctxt mcp` or `ctxt watch` process on a project becomes the writer. Additional servers (`ctxt mcp` in a second AI client, or `ctxt watch` alongside MCP) start in **read-only mode**: searches stay live against their in-memory index, but they do not write the snapshot or synonym cache. This is intended — run several AI clients against one project at once.
- HTTP discovery has a separate owner. The first HTTP server registers its endpoint, including a read-only watcher started after a stdio-only MCP server. Other HTTP servers leave that registration untouched. Closing a server cannot remove another server's registration. If the registered HTTP server stops, restart a remaining HTTP server to register it for CLI memory searches.
- The writer process is the only one that updates the on-disk index (on graceful shutdown). If you stop all servers and later see "another ctxt writer may be active", a previous process was killed rather than stopped; remove the empty `.ctxt-writer` directory in the project root after confirming no ctxt process is running.
- HTTP discovery uses `.ctxt/ctx_runtime.json.lock/` independently of the writer guard. After a crash, confirm all HTTP servers for the project have stopped, remove that empty directory and the stale `.ctxt/ctx_runtime.json`, then restart the HTTP server. Guards are never automatically stolen.

**Stale search results**
- The MCP server watches for changes and keeps its in-memory index current automatically
- If changes aren't picked up, restart the AI client to reload the MCP server

**LLM synonyms not working**
- Search works but conceptual queries are weak
- Set `OPENROUTER_API_KEY` environment variable
- Or add `api_key` to `.ctxt/ctx_config.toml`
- Restart the AI client after setting the key

**Index quality varies**
- The MCP server's index quality depends on the symbol extraction mode chosen at startup
- Use `--symbol-extractor treesitter` for better accuracy on supported languages (Python, JS/TS, Rust, Svelte, Astro)
- Use `--symbol-extractor regex` if tree-sitter is unavailable or for unsupported languages (Vue, Ruby)
- Re-run `ctxt init` with the desired mode before starting the MCP server

**Permission errors**
- The MCP server can't read files or write the index
- Ensure ctxt has read access to your project directory
- Check that `.ctxt/` directory is writable

## Related Commands

- `ctxt init` — Build the index before using MCP
- `ctxt doctor` — Diagnose index and config issues
- `ctxt status` — Check index health from the command line
- `ctxt search-hints` — Test search queries manually
