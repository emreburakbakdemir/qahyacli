package provider

import (
	"fmt"
	"os"
	"strings"
	"gopkg.in/yaml.v3"
)

// file reads secrets from a nested YAML file
// the file is re-read on every lookup. that is deliberate: values are fetched at
// step spawn, so editing the file between steps is picked up by the next step
// without reloading anything
type File struct {
	path string
}

func NewFile(path string) *File { return &File{path: path} }

func (f *File) Lookup(path []string, field string) (string, error) {
	// error messages name the reference, never the value
	ref := strings.Join(path, "/") + "#" + field

	data, err := os.ReadFile(f.path)
	if err != nil { return "", fmt.Errorf("secret %s: %w", ref, err) }

	// walk yaml.Node instead of decoding into map[string]any: the node keeps
	// the original text, so a password like 0123 stays "0123" instead of turning
	// into the number 123
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("secret %s: parse %s: %w", ref, f.path, err)
	}

	node := &doc
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return "", fmt.Errorf("secret %s: %s is empty", ref, f.path)
		}
		node = node.Content[0]
	}

	keys := append(append([]string{}, path...), field)
	for i, key := range keys {
		node = deref(node)
		if node.Kind != yaml.MappingNode {
			return "", fmt.Errorf("secret %s: %q is not a mapping %s",
									ref, strings.Join(keys[:i], "/"), f.path)		
		}
		next := child(node, key)
		if next == nil {
			return "", fmt.Errorf("secret %s: not found in %s", ref, f.path)
		}
		node = next
	}

	node = deref(node)
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("secret %s: not a single value", ref)
	}
	return node.Value, nil
}

// child finds key in a mapping node. mapping content is a flat list:
// [key0, value0, key1, value1, ...]
func child(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i + 1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key { return m.Content[i+1] }
	}
	return nil
}

// deref follows YAML aliases (*anchor) to the node they point at
func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}