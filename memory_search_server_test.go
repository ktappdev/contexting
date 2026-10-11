package contexting

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFormatSearchLogQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		max   int
		want  string
	}{
		{
			name:  "trim and normalize whitespace",
			query: "   auth\tmiddleware\nroutes   ",
			max:   120,
			want:  "auth middleware routes",
		},
		{
			name:  "truncate long query",
			query: "abcdefghijklmnopqrstuvwxyz",
			max:   10,
			want:  "abcdefghij...",
		},
		{
			name:  "invalid max uses default",
			query: "abc",
			max:   0,
			want:  "abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSearchLogQuery(tc.query, tc.max)
			if got != tc.want {
				t.Fatalf("formatSearchLogQuery(%q, %d) = %q, want %q", tc.query, tc.max, got, tc.want)
			}
		})
	}
}

// TestMemorySearchServerDiscoveryOwnership covers discovery independently of
// snapshot ownership, overlapping servers, and repeated concurrent shutdown.
func TestMemorySearchServerDiscoveryOwnership(t *testing.T) {
	root := t.TempDir()
	runtimeFile := filepath.Join(root, "ctx_runtime.json")
	manager := &IndexManager{}

	// A stdio-only MCP process can hold the snapshot guard. The first HTTP
	// server must still register, even though it cannot write snapshots.
	release, err := acquireWriterGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner, err := startMemorySearchServer(ctx, manager, runtimeFile, MemorySearchLogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	state, err := LoadRuntimeState(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if state.PID != os.Getpid() || state.Address != owner.Address() {
		t.Fatalf("incorrect owner registration: %+v", state)
	}

	// Another HTTP server in the same process neither overwrites nor removes
	// the owner's registration. PID alone cannot distinguish these servers.
	observer, err := startMemorySearchServer(ctx, manager, runtimeFile, MemorySearchLogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()
	if err := observer.Close(); err != nil {
		t.Fatal(err)
	}
	state, err = LoadRuntimeState(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if state.Address != owner.Address() {
		t.Fatalf("observer changed owner registration: %+v", state)
	}

	// Cancellation and explicit Close can overlap; every caller must wait
	// until cleanup has completed before a replacement acquires discovery.
	cancel()
	var closes sync.WaitGroup
	for range 8 {
		closes.Add(1)
		go func() {
			defer closes.Done()
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	closes.Wait()
	if _, err := os.Stat(runtimeFile); !os.IsNotExist(err) {
		t.Fatalf("owner did not remove discovery file: %v", err)
	}
	if _, err := os.Stat(runtimeFile + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("owner did not release discovery guard: %v", err)
	}

	replacement, err := startMemorySearchServer(context.Background(), manager, runtimeFile, MemorySearchLogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	_ = owner.Close()
	_ = observer.Close()
	state, err = LoadRuntimeState(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if state.Address != replacement.Address() {
		t.Fatalf("old server changed replacement registration: %+v", state)
	}
}
