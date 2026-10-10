package contexting

import (
	"path"
	"strings"
)

// Expected paths identify exact project-relative entries, including case.
// Normalize separators and dot segments without accepting filename suffixes.
func pathMatchesExpected(resultPath, expected string) bool {
	return expected != "" && normalizeEvalPath(resultPath) == normalizeEvalPath(expected)
}

func normalizeEvalPath(value string) string {
	return path.Clean(strings.ReplaceAll(value, "\\", "/"))
}
