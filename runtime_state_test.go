package contexting

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadRuntimeState(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".ctxt", "ctx_runtime.json")
	state := RuntimeState{
		RootPath:  tmpDir,
		Address:   "127.0.0.1:12345",
		PID:       42,
		StartedAt: time.Now().UTC().Round(time.Second),
	}

	if err := SaveRuntimeState(path, state); err != nil {
		t.Fatalf("save runtime state: %v", err)
	}

	loaded, err := LoadRuntimeState(path)
	if err != nil {
		t.Fatalf("load runtime state: %v", err)
	}
	if loaded.RootPath != state.RootPath || loaded.Address != state.Address || loaded.PID != state.PID {
		t.Fatalf("runtime state mismatch: got %+v want %+v", loaded, state)
	}
}

func TestRuntimeAddressMustBeLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:1234", "[::1]:1234"} {
		if err := validateRuntimeAddress(address); err != nil {
			t.Fatalf("rejected %s: %v", address, err)
		}
	}
	for _, address := range []string{"example.com:80", "192.0.2.1:80", "bad"} {
		if err := validateRuntimeAddress(address); err == nil {
			t.Fatalf("accepted %s", address)
		}
	}
}
