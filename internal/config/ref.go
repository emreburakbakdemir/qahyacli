/*
the parser for secret references. file splits the stored raw reference text into its three
parts ("local:db/prod#password") and checks that the provider exists, returns a small
object that knows how to fetch the value later. this file connects the config package to provider package
*/
package config

import (
	"fmt"
	"qahyacli/internal/provider"
	"strings"
)

// ref is a parsed secret reference bound to its provider. it implements state.Resolver
// so the store can hold it and resolve it at spawn time without knowing anything about config files
type ref struct {
	raw			string
	provider 	provider.Provider
	path 		[]string
	field 		string
}

func (r *ref) Resolve() (string, error) { return r.provider.Lookup(r.path, r.field) }
func (r *ref) String() string			{ return r.raw }

// local:db/prod#password -> provider: local, path: [db, prod], field: "password"
// local:#token -> top-level field "token"
func parseRef(raw string, providers map[string]provider.Provider) (*ref, error) {
	name, rest, ok := strings.Cut(raw, ":")
	if !ok || name == "" {
		return nil, fmt.Errorf("secret %q: expected provider:path#field", raw)
	}
	p, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("secret %q: unkown provider %q", raw, name)
	}

	itemPath, field, ok := strings.Cut(rest, "#")
	if !ok || field == "" {
		// a reference without #field would be a whole group, groups need a mapping to var names and come later
		return nil, fmt.Errorf("secret %q: missing #field", raw)
	}

	var segs []string
	if itemPath != "" {
		segs = strings.Split(itemPath, "/")
		for _, s := range segs {
			if s == "" {
				return nil, fmt.Errorf("secret %q: empty path segment", raw)
			}
		}
	}
	return &ref{raw: raw, provider: p, path: segs, field: field}, nil
}
