package contexting

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResolveStartupConfigPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	const relConfig = ".ctxt/ctx_config.toml"

	// No root arg: anchored to the working directory (previous behavior).
	got, err := resolveStartupConfigPath(relConfig, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cwd, filepath.FromSlash(relConfig)); got != want {
		t.Fatalf("no root arg: got %q, want %q", got, want)
	}

	// Explicit root arg: anchored to the supplied project root.
	// t.TempDir() is absolute on every platform (filepath.Abs adds the
	// current drive on Windows, so a hardcoded "/tmp" would not match).
	absRoot := t.TempDir()
	got, err = resolveStartupConfigPath(relConfig, absRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(absRoot, ".ctxt", "ctx_config.toml"); got != want {
		t.Fatalf("root arg: got %q, want %q", got, want)
	}

	// Relative root arg: resolved against the working directory first.
	got, err = resolveStartupConfigPath(relConfig, "sub/dir", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cwd, "sub", "dir", ".ctxt", "ctx_config.toml"); got != want {
		t.Fatalf("relative root arg: got %q, want %q", got, want)
	}

	// Explicit --config keeps working-directory resolution.
	got, err = resolveStartupConfigPath("other/conf.toml", absRoot, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cwd, "other", "conf.toml"); got != want {
		t.Fatalf("explicit config: got %q, want %q", got, want)
	}

	// Explicit --config with an empty path stays empty.
	got, err = resolveStartupConfigPath("", absRoot, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("empty explicit config: got %q", got)
	}
}

func TestAnchorsConfigToRoot(t *testing.T) {
	// Only watch loads its config anchored to the project root argument,
	// so the bootstrap prompt must anchor there too. Every other command
	// keeps working-directory anchoring (no behavior change).
	for _, name := range []string{"watch"} {
		if !anchorsConfigToRoot(name) {
			t.Fatalf("%s should anchor config to its root arg", name)
		}
	}
	for _, name := range []string{"init", "sync", "search-hints", "bench", "eval", "doctor", "examples", "config"} {
		if anchorsConfigToRoot(name) {
			t.Fatalf("%s should not anchor config to a root arg", name)
		}
	}
}

func TestWriterGuardAllowShared(t *testing.T) {
	root := t.TempDir()
	release, writeMode, err := acquireWriterGuardAllowShared(root)
	if err != nil {
		t.Fatal(err)
	}
	if !writeMode || release == nil {
		t.Fatal("first acquire should be write mode with a release")
	}

	// A second process degrades to read-only instead of failing.
	rel2, writeMode2, err := acquireWriterGuardAllowShared(root)
	if err != nil {
		t.Fatal(err)
	}
	if writeMode2 || rel2 != nil {
		t.Fatal("second acquire should be read-only with no release")
	}

	// The strict writer API still reports the conflict.
	if _, err := acquireWriterGuard(root); err == nil {
		t.Fatal("strict acquire should still fail while the guard is held")
	}

	release()

	// After release the guard is acquirable again.
	rel3, writeMode3, err := acquireWriterGuardAllowShared(root)
	if err != nil {
		t.Fatal(err)
	}
	if !writeMode3 {
		t.Fatal("after release, acquire should be write mode")
	}
	rel3()
}

// newMCPTestServer registers the production MCP tools against an empty in-memory
// index and returns a connected client session, so tool behavior (including input
// validation) is exercised through the real SDK path.
func newMCPTestServer(t *testing.T) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()
	manager := &IndexManager{} // nil index: searches return "No results found."
	server := mcp.NewServer(&mcp.Implementation{Name: "ctxt", Version: "test"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})
	registerMCPSearchTool(server, manager)
	registerMCPStatusTool(server, manager)

	cTransport, sTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, sTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "testClient", Version: "1.0.0"}, nil)
	cs, err := client.Connect(ctx, cTransport, nil)
	if err != nil {
		_ = ss.Close()
		t.Fatal(err)
	}
	return cs, func() {
		_ = cs.Close()
		_ = ss.Close()
	}
}

func TestMCPSearchToolValidation(t *testing.T) {
	cs, cleanup := newMCPTestServer(t)
	defer cleanup()
	ctx := context.Background()

	cases := []struct {
		name string
		args map[string]any
	}{
		{"missing query", map[string]any{}},
		{"blank query", map[string]any{"query": ""}},
		{"invalid type filter", map[string]any{"query": "auth", "type": "symbols"}},
		{"limit above maximum", map[string]any{"query": "auth", "limit": maxMCPResults + 1}},
		{"limit below minimum", map[string]any{"query": "auth", "limit": 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "search", Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			if !res.IsError {
				t.Fatalf("expected validation error for %v, got %+v", tc.args, res)
			}
		})
	}

	t.Run("valid call returns a result", func(t *testing.T) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "search", Arguments: map[string]any{"query": "auth"}})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected error: %+v", res)
		}
		if len(res.Content) == 0 {
			t.Fatal("no content returned")
		}
	})

	t.Run("maximum limit is accepted", func(t *testing.T) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "search", Arguments: map[string]any{"query": "auth", "limit": maxMCPResults}})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("maximum limit should be valid: %+v", res)
		}
	})

	t.Run("search tool is annotated read-only", func(t *testing.T) {
		listed, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range listed.Tools {
			if tool.Name != "search" {
				continue
			}
			if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
				t.Fatalf("search tool missing read-only annotation: %+v", tool.Annotations)
			}
			schema, ok := tool.InputSchema.(map[string]any)
			if !ok {
				t.Fatalf("search tool schema type %T", tool.InputSchema)
			}
			props, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("search tool schema properties %T", schema["properties"])
			}
			typeSchema, ok := props["type"].(map[string]any)
			if !ok {
				t.Fatal("search tool type property missing")
			}
			if enum, ok := typeSchema["enum"]; !ok {
				t.Fatal("search tool type filter has no enum")
			} else if items, ok := enum.([]any); !ok || len(items) != 3 {
				t.Fatalf("search tool type enum: %v", enum)
			}
			limitSchema, ok := props["limit"].(map[string]any)
			if !ok {
				t.Fatal("search tool limit property missing")
			}
			if limitSchema["maximum"] != float64(maxMCPResults) {
				t.Fatalf("search tool limit maximum: %v", limitSchema["maximum"])
			}
			return
		}
		t.Fatal("search tool not advertised")
	})
}

func TestMCPStatusTool(t *testing.T) {
	cs, cleanup := newMCPTestServer(t)
	defer cleanup()
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("status should not error: %+v", res)
	}
	if len(res.Content) == 0 {
		t.Fatal("status returned no content")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("status content type %T", res.Content[0])
	}
	if text.Text == "" || len(text.Text) > 1024 {
		t.Fatalf("unexpected status output: %q", text.Text)
	}
}
