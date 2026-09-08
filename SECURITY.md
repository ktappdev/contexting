# Security and privacy

Contexting is a local developer tool. Run it only against repositories you
trust and with the permissions needed to read that project.

## Data flow

- Indexing reads source files locally to extract symbols and imports.
- When LLM generation is enabled with a configured key, file/folder names
  (including parent directory names), extracted symbols and imports are sent
  to the configured OpenAI-compatible chat-completions endpoint.
- Search queries are matched locally; ctxt does not send queries to an LLM.
  An AI client using MCP has its own data-handling policy.
- Use `--offline` to disable LLM calls regardless of configured keys.
- Default transport is HTTPS OpenRouter. HTTP LLM endpoints are accepted only
  on loopback (`localhost`, `127.0.0.0/8`, or `::1`).
- Query logging is off by default. Existing configurations with
  `watch.search_log = true` continue to log queries; change that value to opt out.
- Indexes, caches and logs can reveal project structure. Keep `.ctxt/` out of
  version control and restrict project-directory access on shared computers.
- API keys are never intentionally printed. Prefer environment variables over
  config files or command-line arguments. Provider errors omit response bodies.

## Local interfaces

MCP uses stdin/stdout. Optional HTTP search binds only to 127.0.0.1 on a random
port. It has no authentication and trusts other processes on the same machine.
Do not forward or proxy this port. Browser-origin search requests are rejected;
request size and timeouts are bounded. The runtime file advertises the port.

The CLI uses an atomic directory guard to exclude concurrent writers for a
project. It is not a distributed lock. Do not share output/cache paths between
different project roots, run writers on network filesystems, or assume the Go
library API acquires this guard. After an unclean exit, verify all project
writers are stopped before removing the empty `.ctxt-writer` directory.

Symlink entries are excluded from indexing. Ignore rules are a convenience,
not a security boundary: review what you index before enabling remote LLMs.

## Reporting

Do not post credentials, proprietary source, or exploitable details in public
issues. Use GitHub's private vulnerability reporting feature if available for
this repository. Otherwise contact the maintainer through the GitHub profile
to arrange a private channel before sending details.

Security fixes target the latest beta release. Older prereleases are not
maintained independently.
