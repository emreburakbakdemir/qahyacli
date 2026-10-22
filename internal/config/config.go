/*
the config loader
*/
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"qahyacli/internal/provider"
	"qahyacli/internal/state"

	"gopkg.in/yaml.v3"
)

type file struct {
	Version			int							`yaml:"version"`	
	Providers 		map[string]providerSpec		`yaml:"providers"`
	Defaults		profile						`yaml:"defaults"`
	Profiles		map[string]profile			`yaml:"profiles"`
}

type providerSpec struct {
	Type	string	`yaml:"type"`
	Path	string	`yaml:"path"`
}

type profile struct {
	Env map[string]value `yaml:"env"`
}

// Load reads the config at path and returns the entries for profileName:
// `defaults` first, the profile on top.
// secret references are checked here and fetched again at spawn: a typo fails
func Load(path, profileName string) (map[string]state.Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // `profils:` is an error, not a silently ignored key
	var f file
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s is empty", path)
		}
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported version %d (expected 1)", path, f.Version)
	}
	providers, err := buildProviders(f.Providers, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	merged := maps.Clone(f.Defaults.Env)
	if merged == nil {
		merged = map[string]value{}
	}

	label := "defaults"
	if profileName != "" {
		p, ok := f.Profiles[profileName]
		if !ok {
			return nil, fmt.Errorf("%s: unkown profile %q (available: %s)",
						path, profileName, strings.Join(slices.Sorted(maps.Keys(f.Profiles)), ", "))
		}
		maps.Copy(merged, p.Env) // a key in the profile replaces the default entirely
		label = profileName
	}

	source := fmt.Sprintf("%s[%s]", filepath.Base(path), label)
	out := make(map[string]state.Entry, len(merged))

	// sorted so that, with several mistakes, the reported one is always the same
	for _, name := range slices.Sorted(maps.Keys(merged)) {
		v := merged[name]
		if err :=state.ValidateName(name); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		if !v.set {
			return nil, fmt.Errorf("%s: %s: empty value; write \"\" for an empty string", path, name)
		}

		e := state.Entry{Value: v.literal, Source: source}
		if v.secret != "" {
			r, err := parseRef(v.secret, providers)
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w",path, name, err)
			}

			// "the check early" half: the value is thrown away here and fetched again when a step is spawned
			if _, err := r.Resolve(); err != nil {
				return nil, fmt.Errorf("%s: %s: %w", path, name, err)
			}
			e.Secret = r
		}
		out[name] = e
	}
	return out, nil
}

func buildProviders(specs map[string]providerSpec, baseDir string) (map[string]provider.Provider, error) {
	out := make(map[string]provider.Provider, len(specs))
	for name, spec := range specs {
		switch spec.Type {
		case "file":
			if spec.Path == "" {
				return nil, fmt.Errorf("provider %q: path is required", name)
			}
			p := spec.Path
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			out[name] = provider.NewFile(p)

		case "":
			return nil, fmt.Errorf("provider %q: type is required", name)
		default:	
			return nil, fmt.Errorf("provider %q: unkown type %q (supported: file)", name, spec.Type)
		}
	}
	return out, nil
}

/*
	Every error Load can produce
	Problem in the config										Where it's caught
	file missing												os.ReadFile
	empty file													io.EOF check
	invalid YAML, unknown top-level key (profils:)				Decode + KnownFields
	FOO: empty, unknown key in { secret: … }, a list as value	value.UnmarshalYAML via Decode
	missing or wrong version									version check
	provider without type or path, unknown type					buildProviders
	unknown profile												profile lookup
	bad name (PATH, QAHYA_X, 2FA)								state.ValidateName (file 3)
	bad reference syntax, unknown provider						parseRef
	secret missing in secrets.yaml								r.Resolve() → file provider (file 5)
*/
