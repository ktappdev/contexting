//go:build !windows

package contexting

import "os"

func replaceFileAtomic(source, destination string) error {
	return os.Rename(source, destination)
}
