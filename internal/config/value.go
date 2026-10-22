/*
	an entry can be written two ways: a plain value or a secref reference. this file teaches
	YAML library how to turn either form into one Go type, value.
*/
package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

/*
	value is one entry under `env:`. two shapes are accepted:
		LOG_LEVEL: info		-> literal
		DB_PASS: { secret: local:db/prod#password } -> reference

	later modifiers (deliver, optional, only, ...) become more keys in the
	mapping form; the literal form stays a plain string
*/
type value struct {
	literal		string
	secret 		string // raw reference, empty for literals
	set			bool   // false when yaml skipped UnmarshalYAML: the value was null (`FOO:`)
}

func (v *value) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		// `FOO:` (null) never gets here: yaml.v3 does not call UnmarshalYAML
		// for null values, it leaves the zero value. config.Load catches that
		// through v.set.

		// n.Value is the text as written. using it instead of letting yaml pick
		// a type is what keeps VERSION: 1.10 from becoming 1.1
		v.literal = n.Value
		v.set = true
		return nil
	
	case yaml.MappingNode:
		for i := 0; i + 1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			switch key.Value {
			case "secret":
				if val.Kind != yaml.ScalarNode || val.Value == "" {
					return fmt.Errorf("line %d: secret must be a reference like provider:path#field", val.Line)
				}
				v.secret = val.Value
			
			default:
				return fmt.Errorf("line %d: unknown key %q (supported: secret)", key.Line, key.Value)	
			}
		}
		if v.secret == "" {
			return fmt.Errorf("line %d: expected { secret: provider:path#field }", n.Line)
		}
		v.set = true
		return nil

	default:
		return fmt.Errorf("line %d: value must be a string or { secret: ... }", n.Line)	
	}
}