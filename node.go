package main

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
	Children map[string]*Node `json:"children,omitempty"`
}

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

func traverseFolder(rootPath string, ignoredPaths map[string]bool) (*Node, error) {
	return BuildTree(rootPath, ignoredPaths)
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
		name := filepath.Base(node.FullPath)
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
			name := filepath.Base(node.FullPath)
			node.Synonyms = sanitizeSynonyms(lexicalSynonyms(name), maxPerNode)
		})
		return
	}

	walkTree(tree, func(node *Node) {
		if node == tree {
			return
		}
		name := filepath.Base(node.FullPath)
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
