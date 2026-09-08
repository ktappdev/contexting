# Contributing

Use Go 1.25 or newer. Keep changes small and include regression coverage for
behavior changes. Run:

```sh
go test ./...
go test -race ./...
go vet ./...
staticcheck ./...
golangci-lint run
govulncheck ./...
```

The CI workflow installs pinned analysis tools. Tests use local fake LLM
servers and do not require API credentials. Full tests build a CLI and exercise
MCP and watch subprocesses; `go test -short ./...` skips that lifecycle test.

Do not commit generated indexes, binaries, API keys or private fixtures.
See SECURITY.md for data handling and AGENTS.md for architecture.
