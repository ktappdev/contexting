package main

import (
	"os"
	"path/filepath"
)

type Node struct {
	FullPath string
	Type     string
	Synonyms []string
	Children map[string]*Node
}

func traverseFolder(rootPath string, ignoredPaths map[string]bool) (*Node, error) {
	root := &Node{
		FullPath: rootPath,
		Type:     "directory",
		Children: make(map[string]*Node),
	}

	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		if ignoredPaths[rel] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		nodeType := "file"
		if d.IsDir() {
			nodeType = "directory"
		}

		parent := root
		parts := filepath.SplitList(rel)
		for i := 0; i < len(parts)-1; i++ {
			child, ok := parent.Children[parts[i]]
			if !ok {
				return nil
			}
			parent = child
		}

		name := filepath.Base(path)
		parent.Children[name] = &Node{
			FullPath: path,
			Type:     nodeType,
			Children: make(map[string]*Node),
		}

		return nil
	})

	return root, err
}

func CollectNamesForLLM(tree *Node) []string {
	var names []string
	collectNames(tree, &names)
	return names
}

func collectNames(node *Node, names *[]string) {
	if node.FullPath != "" && node.FullPath != "." {
		name := filepath.Base(node.FullPath)
		*names = append(*names, name)
	}

	for _, child := range node.Children {
		collectNames(child, names)
	}
}
