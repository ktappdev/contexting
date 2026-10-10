package contexting

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func benchExecutable(name string) (string, error) {
	binary, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s binary not found in PATH", name)
	}
	return binary, nil
}

// The saved tree defines the shared file corpus, including its ignore policy.
// Reject stale entries instead of comparing different sets of files silently.
func benchIndexedFiles(index *ContextIndex, maxBytes int) (map[string]string, error) {
	files := make(map[string]string)
	if index == nil || index.Tree == nil {
		return files, fmt.Errorf("benchmark requires a file tree")
	}
	directories := make(map[string]bool)
	var invalid error
	walkTree(index.Tree, func(node *Node) {
		if invalid != nil {
			return
		}
		rel, err := filepath.Rel(index.Tree.FullPath, node.FullPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			invalid = fmt.Errorf("indexed path outside project root: %s", node.FullPath)
			return
		}
		path := filepath.Join(index.RootPath, rel)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || (node != index.Tree && !directories[filepath.Dir(path)]) {
			invalid = fmt.Errorf("indexed path unavailable or replaced by a symlink: %s; rebuild with ctxt init", path)
			return
		}
		if node.Type == "directory" && info.IsDir() {
			directories[path] = true
			return
		}
		if node.Type == "file" && info.Mode().IsRegular() {
			if node.ModTime != 0 && (node.ModTime != info.ModTime().UnixNano() || node.Size != info.Size()) {
				invalid = fmt.Errorf("indexed file changed: %s; rebuild with ctxt init", path)
				return
			}
			if maxBytes <= 0 || info.Size() <= int64(maxBytes) {
				files[path] = filepath.ToSlash(rel)
			}
		} else {
			invalid = fmt.Errorf("indexed entry changed type: %s; rebuild with ctxt init", path)
		}
	})
	return files, invalid
}

func benchCommand(name, root string, args []string) ([]string, error) {
	binary, err := benchExecutable(name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = root
	output, err := command.Output()
	if err != nil && !((name == "grep" || name == "rg") && isExitCode1(err)) {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s command: %w", name, ctx.Err())
		}
		return nil, fmt.Errorf("%s command failed: %w", name, err)
	}
	var paths []string
	for _, value := range strings.Split(string(output), "\x00") {
		if value != "" {
			paths = append(paths, filepath.Clean(value))
		}
	}
	return paths, nil
}

func benchFilenameSearch(name, query string, expected []string, index *ContextIndex, opts SearchOptions, prepared BenchCorpus) EngineResult {
	start := time.Now()
	corpus := prepared.files
	args := []string{index.RootPath, "-type", "f", "-print0"}
	if name == "fd" {
		args = []string{"--absolute-path", "--type", "f", "--hidden", "--no-ignore", "--print0", "--search-path", index.RootPath}
	}
	candidates, err := benchCommand(name, index.RootPath, args)
	var paths []string
	tokens := tokenize(query)
	for _, candidate := range candidates {
		if rel, ok := corpus[candidate]; ok && tokensMatchPath(rel, tokens) {
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	return benchResult(name, paths, expected, opts.Limit, start, err)
}

func benchContentSearch(name, query string, expected []string, index *ContextIndex, opts SearchOptions, maxBytes int, prepared BenchCorpus) EngineResult {
	start := time.Now()
	if _, err := benchExecutable(name); err != nil {
		return benchResult(name, nil, expected, opts.Limit, start, err)
	}
	corpus := prepared.contentFiles
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return benchResult(name, nil, expected, opts.Limit, start, nil)
	}
	args := []string{"-I", "-l", "-i", "-F", "--null"}
	if name == "rg" {
		args = []string{"--files-with-matches", "--null", "--ignore-case", "--fixed-strings", "--no-ignore"}
	}
	for _, token := range tokens {
		args = append(args, "-e", token)
	}
	args = append(args, "--")
	files := make([]string, 0, len(corpus))
	for file := range corpus {
		files = append(files, file)
	}
	sort.Strings(files)
	var paths []string
	// Bound argv size on supported platforms; each command uses literal patterns.
	for start := 0; start < len(files); {
		end, bytes := start, 0
		for end < len(files) && (end == start || bytes+len(files[end])+1 <= 8000) {
			bytes += len(files[end]) + 1
			end++
		}
		batch := append(append([]string(nil), args...), files[start:end]...)
		matches, err := benchCommand(name, index.RootPath, batch)
		if err != nil {
			return benchResult(name, nil, expected, opts.Limit, time.Now(), err)
		}
		for _, file := range matches {
			if rel, ok := corpus[file]; ok {
				paths = append(paths, rel)
			}
		}
		start = end
	}
	sort.Strings(paths)
	return benchResult(name, paths, expected, opts.Limit, start, nil)
}

func tokensMatchPath(path string, tokens []string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}
