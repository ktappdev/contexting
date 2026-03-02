package main

import (
	"os"
	"path/filepath"
	"strings"
)

func removeNodeByRelPath(root *Node, relPath string) bool {
	if root == nil {
		return false
	}
	parts := splitRelPath(relPath)
	if len(parts) == 0 {
		return false
	}

	parent := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := parent.Children[parts[i]]
		if !ok {
			return false
		}
		parent = next
	}

	name := parts[len(parts)-1]
	if _, ok := parent.Children[name]; !ok {
		return false
	}
	delete(parent.Children, name)
	return true
}

func upsertNodeByRelPath(root *Node, absRoot string, relPath string, isDir bool, cache SynonymResponse, maxSynonyms int) bool {
	if root == nil {
		return false
	}
	parts := splitRelPath(relPath)
	if len(parts) == 0 {
		return false
	}

	changed := false
	parent := root
	currPath := absRoot

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		currPath = filepath.Join(currPath, part)
		next, ok := parent.Children[part]
		if !ok {
			next = &Node{
				FullPath: currPath,
				Type:     "directory",
				Children: make(map[string]*Node),
			}
			parent.Children[part] = next
			next.Synonyms = buildNodeSynonyms(part, cache, maxSynonyms)
			changed = true
		}
		if next.Type != "directory" {
			next.Type = "directory"
			changed = true
		}
		if next.Children == nil {
			next.Children = make(map[string]*Node)
			changed = true
		}
		parent = next
	}

	name := parts[len(parts)-1]
	fullPath := filepath.Join(absRoot, filepath.FromSlash(strings.Join(parts, "/")))
	nodeType := "file"
	if isDir {
		nodeType = "directory"
	}
	nodeSynonyms := buildNodeSynonyms(name, cache, maxSynonyms)

	node, ok := parent.Children[name]
	if !ok {
		parent.Children[name] = &Node{
			FullPath: fullPath,
			Type:     nodeType,
			Synonyms: nodeSynonyms,
			Children: make(map[string]*Node),
		}
		return true
	}

	if node.FullPath != fullPath {
		node.FullPath = fullPath
		changed = true
	}
	if node.Type != nodeType {
		node.Type = nodeType
		changed = true
	}
	if node.Children == nil {
		node.Children = make(map[string]*Node)
		changed = true
	}
	if !stringSlicesEqual(node.Synonyms, nodeSynonyms) {
		node.Synonyms = nodeSynonyms
		changed = true
	}

	return changed
}

func buildNodeSynonyms(name string, cache SynonymResponse, maxSynonyms int) []string {
	combined := make([]string, 0, maxSynonyms+4)
	if cache != nil {
		if syns, ok := cache[name]; ok {
			combined = append(combined, syns...)
		}
	}
	combined = append(combined, lexicalSynonyms(name)...)
	return sanitizeSynonyms(combined, maxSynonyms)
}

func splitRelPath(relPath string) []string {
	normalized := strings.TrimSpace(filepath.ToSlash(relPath))
	normalized = strings.Trim(normalized, "/")
	if normalized == "" || normalized == "." {
		return nil
	}
	parts := strings.Split(normalized, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		out = append(out, part)
	}
	return out
}

func isExistingDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
