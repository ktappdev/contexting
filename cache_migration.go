package contexting

import "path/filepath"

// migrateSynonymCache preserves only legacy entries that identify one node.
// Ambiguous suffixes must not carry one file's meaning into another file.
func migrateSynonymCache(tree *Node, state synonymCacheState) synonymCacheState {
	counts := make(map[string]int)
	walkTree(tree, func(node *Node) {
		if node == tree {
			return
		}
		counts[legacySynonymKey(node)]++
	})
	entries := make(SynonymResponse)
	walkTree(tree, func(node *Node) {
		if node == tree {
			return
		}
		rel, err := filepath.Rel(tree.FullPath, node.FullPath)
		if err != nil {
			return
		}
		node.SynonymKey = filepath.ToSlash(rel)
		if state.Version >= 2 {
			return
		}
		legacy := legacySynonymKey(node)
		if counts[legacy] == 1 {
			if values, ok := state.Entries[legacy]; ok {
				entries[node.SynonymKey] = values
				return
			}
		}
		// Support callers already supplying relative keys in the public map API.
		if counts[node.SynonymKey] == 0 {
			if values, ok := state.Entries[node.SynonymKey]; ok {
				entries[node.SynonymKey] = values
			}
		}
	})
	if state.Version < 2 {
		state.Entries = entries
	}
	state.Version = 2
	if state.Fingerprints == nil {
		state.Fingerprints = make(map[string]string)
	}
	return state
}

func legacySynonymKey(node *Node) string {
	if node.Type == "file" {
		return pathSuffix(node.FullPath)
	}
	return filepath.Base(node.FullPath)
}
