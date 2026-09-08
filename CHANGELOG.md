# Changelog

## 0.1.0-beta.1 — Unreleased

- Consistent parent/file synonym keys during indexing and live updates.
- Watch detects children of newly populated directories.
- Explicit shutdown-only persistence contract; unsupported modes fail.
- HTTPS public LLM defaults, opt-in query logs and an explicit offline mode.
- Remote LLM endpoints require HTTPS; loopback development endpoints may use HTTP.
- Config-aware API-key diagnostics without printing key fragments.
- Bounded, cancellable transient HTTP retries and reported partial failures.
- Project single-writer guard and versioned index schema.
- File-limit failures instead of silently truncated initial indexes.
- Symlink exclusion, bounded local HTTP requests and atomic file syncing.
- CLI/watch/MCP regression tests and cross-platform CI.
- Versioned platform archives, SHA-256 checksums and draft release workflow.

### Upgrading

Back up `.ctxt/` before upgrading. Unversioned indexes (schema 0) remain readable;
the next save writes schema 1. Unknown newer schemas fail with an upgrade/rebuild
message. Run `ctxt init . --offline` to rebuild; the synonym cache is reused.
Search ranking and JSON output may evolve during the beta series.

Existing config files are not rewritten. Replace private HTTP defaults if you
want OpenRouter, set `watch.search_log = false`, and use `persist = "shutdown"`.
Remove obsolete interval settings. Repeated parent/file suffixes can still
share synonym cache entries; this key format is retained for compatibility.
