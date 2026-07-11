package contexting

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Node struct {
	FullPath string           `json:"full_path"`
	Type     string           `json:"type"`
	Synonyms []string         `json:"synonyms,omitempty"`
	Symbols  []string         `json:"symbols,omitempty"`
	ModTime  int64            `json:"mod_time,omitempty"`
	Children map[string]*Node `json:"children,omitempty"`
}

const MaxFileCount = 10000 // Safety limit to prevent crashes on large repos

type IndexStats struct {
	TotalNodes     int `json:"total_nodes"`
	TotalFiles     int `json:"total_files"`
	TotalDirs      int `json:"total_dirs"`
	SynonymNodes   int `json:"synonym_nodes"`
	CollectedNames int `json:"collected_names"`
}

func BuildTree(rootPath string, ignored map[string]bool) (*Node, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	root := &Node{
		FullPath: absRoot,
		Type:     "directory",
		Children: make(map[string]*Node),
	}

	fileCount := 0
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if shouldIgnorePath(rel, d.Name(), ignored) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		parent := root
		parentRel := filepath.Dir(rel)
		if parentRel != "." {
			parts := strings.Split(parentRel, string(os.PathSeparator))
			for _, part := range parts {
				next, ok := parent.Children[part]
				if !ok {
					return fmt.Errorf("missing parent node for %q", path)
				}
				parent = next
			}
		}

		nodeType := "file"
		if d.IsDir() {
			nodeType = "directory"
		} else {
			fileCount++
			if fileCount > MaxFileCount {
				LogWarnf("File count limit reached (%d). Partial index built with what was scanned. Add ignore patterns to reduce file count.", MaxFileCount)
				return filepath.SkipAll
			}
			if fileCount == MaxFileCount/2 {
				LogWarnf("Large repository detected (%d files). Consider adding more ignore patterns.", fileCount)
			}
		}

		name := d.Name()
		parent.Children[name] = &Node{
			FullPath: path,
			Type:     nodeType,
			Children: make(map[string]*Node),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return root, nil
}

func pathSuffix(fullPath string) string {
	dir := filepath.Dir(fullPath)
	base := filepath.Base(fullPath)
	parent := filepath.Base(dir)
	// For root-level files, parent is the project directory name (still useful context).
	// For deeply nested files, parentDir/basename is usually unique enough.
	return parent + "/" + base
}

// llmSynonymKey returns the key used for LLM synonym generation and lookup.
// Files use parentDir/basename (pathSuffix); directories use just basename.
// Must match CollectNamesForLLM's key logic.
func llmSynonymKey(node *Node) string {
	if node.Type == "file" {
		return pathSuffix(node.FullPath)
	}
	return filepath.Base(node.FullPath)
}

func CollectNamesForLLM(tree *Node) []string {
	if tree == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var names []string
	walkTree(tree, func(node *Node) {
		if node == tree {
			return
		}
		name := llmSynonymKey(node)
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	})

	sort.Strings(names)
	return names
}

func AssignSynonymsToTree(tree *Node, synonyms SynonymResponse, maxPerNode int) {
	if tree == nil || len(synonyms) == 0 {
		walkTree(tree, func(node *Node) {
			if node == tree {
				return
			}
			name := llmSynonymKey(node)
			node.Synonyms = sanitizeSynonyms(lexicalSynonyms(name), maxPerNode)
		})
		return
	}

	walkTree(tree, func(node *Node) {
		if node == tree {
			return
		}
		name := llmSynonymKey(node)
		combined := make([]string, 0, maxPerNode+4)
		if syns, ok := synonyms[name]; ok {
			combined = append(combined, syns...)
		}
		combined = append(combined, lexicalSynonyms(name)...)
		node.Synonyms = sanitizeSynonyms(combined, maxPerNode)
	})
}

func ComputeStats(tree *Node) IndexStats {
	stats := IndexStats{}
	if tree == nil {
		return stats
	}

	walkTree(tree, func(node *Node) {
		stats.TotalNodes++
		if node.Type == "directory" {
			stats.TotalDirs++
		} else {
			stats.TotalFiles++
		}
		if len(node.Synonyms) > 0 {
			stats.SynonymNodes++
		}
	})

	return stats
}

func walkTree(node *Node, fn func(*Node)) {
	if node == nil {
		return
	}

	fn(node)

	keys := make([]string, 0, len(node.Children))
	for name := range node.Children {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	for _, key := range keys {
		walkTree(node.Children[key], fn)
	}
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
