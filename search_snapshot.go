package contexting

// cloneSearchTree isolates search from watch mutations. Only copying happens
// under the manager lock; scoring and external content searches run unlocked.
func cloneSearchTree(node *Node) *Node {
	if node == nil {
		return nil
	}
	snapshot := *node
	snapshot.Synonyms = append([]string(nil), node.Synonyms...)
	snapshot.Symbols = append([]string(nil), node.Symbols...)
	snapshot.Children = make(map[string]*Node, len(node.Children))
	for name, child := range node.Children {
		snapshot.Children[name] = cloneSearchTree(child)
	}
	return &snapshot
}
