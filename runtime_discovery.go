package contexting

import (
	"fmt"
	"os"
	"path/filepath"
)

// Runtime discovery has its own owner: a stdio-only MCP server may hold the
// snapshot writer guard without providing an HTTP endpoint.
type runtimeRegistration struct {
	path      string
	guardPath string
	state     RuntimeState
}

func registerRuntimeState(path string, state RuntimeState) (*runtimeRegistration, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create discovery directory: %w", err)
	}
	guardPath := path + ".lock"
	if err := os.Mkdir(guardPath, 0o700); err != nil {
		if os.IsExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("acquire discovery guard: %w", err)
	}
	if err := SaveRuntimeState(path, state); err != nil {
		_ = os.Remove(guardPath)
		return nil, err
	}
	return &runtimeRegistration{path: path, guardPath: guardPath, state: state}, nil
}

// The discovery guard stays held through cleanup, so another server cannot
// replace the registration between checking ownership and removing the file.
func (r *runtimeRegistration) close() {
	if r == nil {
		return
	}
	state, err := LoadRuntimeState(r.path)
	if err == nil && state.PID == r.state.PID && state.Address == r.state.Address && state.StartedAt.Equal(r.state.StartedAt) {
		_ = os.Remove(r.path)
	}
	_ = os.Remove(r.guardPath)
}
