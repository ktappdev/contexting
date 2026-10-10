package contexting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type synonymContext struct {
	Symbols      map[string][]string
	Imports      map[string][]string
	Fingerprints map[string]string
}

// Fingerprints describe the exact local context supplied to the generator.
func collectSynonymContext(tree *Node) synonymContext {
	result := synonymContext{Symbols: make(map[string][]string), Imports: make(map[string][]string), Fingerprints: make(map[string]string)}
	walkTree(tree, func(node *Node) {
		if node == tree {
			return
		}
		name := node.SynonymKey
		if node.Type == "file" {
			result.Symbols[name] = append([]string(nil), node.Symbols...)
			result.Imports[name] = extractFileImports(node.FullPath)
			sort.Strings(result.Symbols[name])
			sort.Strings(result.Imports[name])
		}
		data, _ := json.Marshal(struct {
			Type    string
			Symbols []string
			Imports []string
		}{node.Type, result.Symbols[name], result.Imports[name]})
		sum := sha256.Sum256(data)
		result.Fingerprints[name] = hex.EncodeToString(sum[:])
	})
	return result
}

func namesNeedingSynonyms(names []string, cache SynonymResponse, previous, current map[string]string, minimum int) []string {
	var targets []string
	for _, name := range names {
		if len(cache[name]) < minimum || previous[name] != current[name] {
			targets = append(targets, name)
		}
	}
	return targets
}

// A sync must not stamp stale symbols as current when extraction fails.
func refreshSynonymSymbols(tree *Node, names []string) error {
	selected := make(map[string]bool, len(names))
	for _, name := range names {
		selected[name] = true
	}
	var extractionErr error
	walkTree(tree, func(node *Node) {
		if extractionErr != nil || node.Type != "file" || !selected[node.SynonymKey] {
			return
		}
		symbols, err := extractSymbols(node.FullPath)
		if err != nil {
			extractionErr = fmt.Errorf("extract symbols for sync %s: %w", node.FullPath, err)
			return
		}
		node.Symbols = symbols
	})
	return extractionErr
}
