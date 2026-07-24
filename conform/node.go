package conform

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// The tool parses both OpenAPI documents and live JSON bodies through
// yaml.Node (JSON is a YAML subset), which keeps every value behind a typed
// node instead of interface{} — BFD-16 applies to this codebase too.

type nodeEntry struct {
	Key   string
	Value *yaml.Node
}

// nodeEntries returns the key/value pairs of a mapping node, in order.
// Non-mapping nodes yield nil.
func nodeEntries(node *yaml.Node) []nodeEntry {
	node = nodeUnwrap(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	entries := make([]nodeEntry, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		entries = append(entries, nodeEntry{Key: node.Content[i].Value, Value: node.Content[i+1]})
	}
	return entries
}

type nodeGetInput struct {
	Node *yaml.Node
	Key  string
}

// nodeGet returns the value for a key in a mapping node, or nil.
func nodeGet(input nodeGetInput) *yaml.Node {
	for _, entry := range nodeEntries(input.Node) {
		if entry.Key == input.Key {
			return entry.Value
		}
	}
	return nil
}

// nodeUnwrap steps through document and alias indirection to the real node.
func nodeUnwrap(node *yaml.Node) *yaml.Node {
	for node != nil {
		if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
			node = node.Content[0]
			continue
		}
		if node.Kind == yaml.AliasNode && node.Alias != nil {
			node = node.Alias
			continue
		}
		return node
	}
	return nil
}

// nodeScalarKind names the JSON kind of a scalar node.
func nodeScalarKind(node *yaml.Node) string {
	switch node.Tag {
	case "!!bool":
		return "bool"
	case "!!int", "!!float":
		return "number"
	case "!!null":
		return "null"
	default:
		return "string"
	}
}

// nodeWalkEvent describes one value met while walking a parsed document.
type nodeWalkEvent struct {
	Path  string // dotted path from the root, e.g. "data[0].updatedAt"
	Key   string // the mapping key owning this value; "" for the root and array items
	Kind  string // "object", "array", "string", "number", "bool", "null"
	Value string // scalar text; "" for objects and arrays
}

type nodeWalkInput struct {
	Node    *yaml.Node
	Path    string
	Key     string
	OnEvent func(event nodeWalkEvent)
}

// nodeWalk visits every value in the tree, depth first, in document order.
func nodeWalk(input nodeWalkInput) {
	node := nodeUnwrap(input.Node)
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		input.OnEvent(nodeWalkEvent{Path: input.Path, Key: input.Key, Kind: "object"})
		for _, entry := range nodeEntries(node) {
			childPath := entry.Key
			if input.Path != "" {
				childPath = input.Path + "." + entry.Key
			}
			nodeWalk(nodeWalkInput{Node: entry.Value, Path: childPath, Key: entry.Key, OnEvent: input.OnEvent})
		}
	case yaml.SequenceNode:
		input.OnEvent(nodeWalkEvent{Path: input.Path, Key: input.Key, Kind: "array"})
		for i, item := range node.Content {
			nodeWalk(nodeWalkInput{
				Node:    item,
				Path:    fmt.Sprintf("%s[%d]", input.Path, i),
				OnEvent: input.OnEvent,
			})
		}
	case yaml.ScalarNode:
		input.OnEvent(nodeWalkEvent{
			Path:  input.Path,
			Key:   input.Key,
			Kind:  nodeScalarKind(node),
			Value: node.Value,
		})
	}
}
