package contexting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriterGuardConflictError reports that another ctxt writer owns the guard
// directory for the project.
type WriterGuardConflictError struct {
	GuardPath string
}

func (e *WriterGuardConflictError) Error() string {
	return fmt.Sprintf("another ctxt writer may be active: %s; after verifying all ctxt writers have stopped, remove this empty directory and retry", e.GuardPath)
}

// acquireWriterGuard serializes CLI writers using atomic directory creation.
// It deliberately never steals a guard: after a crash the operator must verify
// no writer is running before removing the empty guard directory.
func acquireWriterGuard(root string) (func(), error) {
	path, err := writerGuardPath(root)
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		if os.IsExist(err) {
			return nil, &WriterGuardConflictError{GuardPath: path}
		}
		return nil, fmt.Errorf("acquire writer guard: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil
}

func writerGuardPath(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, ".ctxt-writer"), nil
}

// acquireWriterGuardAllowShared behaves like acquireWriterGuard, but when another
// process already holds the guard it degrades to read-only observer mode instead of
// failing. Long-running servers (ctxt watch, ctxt mcp) use this so several processes
// can serve in-memory searches against the same project concurrently: in-memory
// updates stay live, while the snapshot and synonym cache are written only by the
// single guard holder. In read-only mode release is nil and the caller must not flush.
func acquireWriterGuardAllowShared(root string) (release func(), writeMode bool, err error) {
	release, err = acquireWriterGuard(root)
	if err != nil {
		var conflict *WriterGuardConflictError
		if errors.As(err, &conflict) {
			LogWarnf("%s", conflict.Error())
			LogWarnf("Another ctxt process holds the writer guard. Running in read-only mode: in-memory search stays live, but snapshot and cache writes are disabled.")
			return nil, false, nil
		}
		return nil, false, err
	}
	return release, true, nil
}
