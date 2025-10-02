package menu

import "strings"

// Node represents a menu entry definition within the registry tree.
type Node struct {
	ID          string
	Loader      Loader
	Action      Action
	Children    map[string]*Node
	MultiSelect bool
}

// Registry exposes lookup utilities for menu definitions.
type Registry struct {
	root  *Node
	nodes map[string]*Node
}

// BuildRegistry constructs the registry from existing loader/handler maps.
func BuildRegistry() *Registry {
	nodes := make(map[string]*Node)

	ensure := func(id string) *Node {
		if node, ok := nodes[id]; ok {
			return node
		}
		node := &Node{ID: id, Children: make(map[string]*Node)}
		nodes[id] = node
		return node
	}

	root := ensure("root")
	root.Loader = func(Context) ([]Item, error) { return RootItems(), nil }

	for id, loader := range CategoryLoaders() {
		node := ensure(id)
		node.Loader = loader
	}

	for id, loader := range ActionLoaders() {
		node := ensure(id)
		node.Loader = loader
	}

	for id, action := range ActionHandlers() {
		node := ensure(id)
		node.Action = action
	}

	markMultiSelect := []string{
		"window:kill",
		"pane:join",
		"pane:kill",
	}
	for _, id := range markMultiSelect {
		if node, ok := nodes[id]; ok {
			node.MultiSelect = true
		}
	}

	for id, node := range nodes {
		if id == "root" {
			continue
		}
		parentID, key := parentKey(id)
		parent := ensure(parentID)
		parent.Children[key] = node
	}

	return &Registry{root: root, nodes: nodes}
}

// Root returns the registry root node.
func (r *Registry) Root() *Node {
	return r.root
}

// Find locates a node by ID.
func (r *Registry) Find(id string) (*Node, bool) {
	node, ok := r.nodes[id]
	return node, ok
}

// Child resolves a child node under the given parent for the provided key.
func (r *Registry) Child(parentID, key string) (*Node, bool) {
	parent, ok := r.nodes[parentID]
	if !ok {
		return nil, false
	}
	node, ok := parent.Children[key]
	return node, ok
}

func parentKey(id string) (string, string) {
	if id == "" {
		return "root", ""
	}
	if !strings.Contains(id, ":") {
		return "root", id
	}
	idx := strings.LastIndex(id, ":")
	return id[:idx], id[idx+1:]
}
