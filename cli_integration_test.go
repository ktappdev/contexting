package contexting

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCLILifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "ctxt")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/ctxt")
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("payment.go", "package app\nfunc Invoice() {}\n")
	command := func(args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, binary, append([]string{"--offline", "--no-config-prompt"}, args...)...)
		cmd.Dir = root
		return cmd
	}
	version := exec.CommandContext(ctx, binary, "version")
	version.Dir = root
	if data, err := version.CombinedOutput(); err != nil || !strings.Contains(string(data), "ctxt version") {
		t.Fatalf("version: %v\n%s", err, data)
	}
	run := func(args ...string) []byte {
		t.Helper()
		data, err := command(args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, data)
		}
		return data
	}
	run("config", "init")
	run("init", ".")
	if data := run("search-hints", "Invoice", "--memory=false", "--json"); !json.Valid(data) || !strings.Contains(string(data), "payment.go") {
		t.Fatalf("snapshot search: %s", data)
	}
	if _, err := command("watch", ".", "--persist=interval").CombinedOutput(); err == nil {
		t.Fatal("accepted unsupported persistence")
	}

	// MCP exercises a real stdio handshake, discovery, both tools and EOF shutdown
	// on every platform, including Windows where os.Interrupt is unavailable.
	client := mcp.NewClient(&mcp.Implementation{Name: "ctxt-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command("mcp", ".", "--llm-on-watch=false", "--debounce=20ms")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	listed, err := session.ListTools(ctx, nil)
	if err != nil || len(listed.Tools) != 2 {
		t.Fatalf("tools: %v %v", listed, err)
	}
	for _, name := range []string{"status", "search"} {
		args := map[string]any{}
		if name == "search" {
			args["query"] = "Invoice"
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("%s: %v %v", name, result, err)
		}
	}
	write("payment.go", "package app\nfunc RefundPayment() {}\n")
	deadline := time.Now().Add(10 * time.Second)
	for {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "search", Arguments: map[string]any{"query": "RefundPayment"}})
		data, _ := json.Marshal(result)
		if err == nil && strings.Contains(string(data), "payment.go") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("MCP did not observe update: %s %v", data, err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if data := run("search-hints", "RefundPayment", "--memory=false", "--json"); !strings.Contains(string(data), "payment.go") {
		t.Fatalf("MCP shutdown snapshot: %s", data)
	}

	if runtime.GOOS == "windows" {
		return
	}
	watch := command("watch", ".", "--llm-on-watch=false", "--debounce=20ms")
	if err := watch.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- watch.Wait() }()
	t.Cleanup(func() { _ = watch.Process.Kill() })
	deadline = time.Now().Add(10 * time.Second)
	for {
		if _, err := LoadRuntimeState(filepath.Join(root, ".ctxt", "ctx_runtime.json")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("watch did not start")
		}
		time.Sleep(30 * time.Millisecond)
	}
	if _, err := command("init", ".").CombinedOutput(); err == nil {
		t.Fatal("concurrent init was allowed")
	}
	write("payment.go", "package app\nfunc ReconcileLedger() {}\n")
	deadline = time.Now().Add(10 * time.Second)
	for {
		data, err := command("search-hints", "ReconcileLedger", "--memory-only", "--json").CombinedOutput()
		if err == nil && strings.Contains(string(data), "payment.go") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("watch update: %s %v", data, err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	if err := watch.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("watch failed to stop")
	}
	if data := run("search-hints", "ReconcileLedger", "--memory=false", "--json"); !strings.Contains(string(data), "payment.go") {
		t.Fatalf("watch shutdown snapshot: %s", data)
	}
}
