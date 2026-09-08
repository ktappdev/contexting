package contexting

import (
	"fmt"
	"os"
	"path/filepath"
)

// acquireWriterGuard serializes CLI writers using atomic directory creation.
// It deliberately never steals a guard: after a crash the operator must verify
// no writer is running before removing the empty guard directory.
func acquireWriterGuard(root string) (func(), error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(abs, ".ctxt-writer")
	if err := os.Mkdir(path, 0o700); err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another ctxt writer may be active: %s; after verifying all ctxt writers have stopped, remove this empty directory and retry", path)
		}
		return nil, fmt.Errorf("acquire writer guard: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil
}
