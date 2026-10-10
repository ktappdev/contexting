package contexting

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// searchIndexedContent searches explicit indexed files, so ripgrep cannot add
// ignored files or apply a different ignore policy to whitelisted dot files.
func searchIndexedContent(index *ContextIndex, tokens []string, opts SearchOptions) []SearchResult {
	score := opts.ContentMatchScore
	if score <= 0 {
		score = 1
	}
	if score < opts.MinScore {
		return nil
	}
	root := opts.ContentRoot
	if root == "" {
		root = index.RootPath
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil
	}

	paths := make([]string, 0)
	relPaths := make(map[string]string)
	validDirs := make(map[string]bool)
	walkTree(index.Tree, func(node *Node) {
		rel, err := filepath.Rel(index.Tree.FullPath, node.FullPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return
		}
		path := filepath.Join(root, rel)
		if node != index.Tree && !validDirs[filepath.Dir(path)] {
			return
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return
		}
		if node.Type == "directory" && info.IsDir() {
			validDirs[path] = true
		} else if node.Type == "file" && info.Mode().IsRegular() {
			paths = append(paths, path)
			relPaths[path] = rel
		}
	})

	args := []string{"--files-with-matches", "--null", "--fixed-strings", "--ignore-case"}
	for _, token := range tokens {
		args = append(args, "-e", token)
	}
	args = append(args, "--")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := make([]SearchResult, 0)
	// Bound argument size for Windows and repositories with long file paths.
	for start := 0; start < len(paths); {
		end, bytes := start, 0
		for end < len(paths) && (end == start || bytes+len(paths[end])+1 <= 8000) {
			bytes += len(paths[end]) + 1
			end++
		}
		batchArgs := append(append([]string(nil), args...), paths[start:end]...)
		out, err := exec.CommandContext(ctx, "rg", batchArgs...).Output()
		if err != nil && !isExitCode1(err) {
			return results
		}
		for _, path := range strings.Split(string(out), "\x00") {
			rel, ok := relPaths[filepath.Clean(path)]
			if !ok {
				continue
			}
			result := SearchResult{Path: rel, Type: "file", Score: score, Matches: []string{"content"}}
			if opts.IncludeDebug {
				result.Breakdown = []string{"content match via ripgrep"}
			}
			results = append(results, result)
		}
		start = end
	}
	return results
}

func isExitCode1(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	return ok && exitErr.ExitCode() == 1
}
