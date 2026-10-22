// this file decides which names are allowed before anything reaches the store.
// it has two callers, the job API server when a step runs and the config loader
// for every name under env: in abc.config
package state

import (
	"fmt"
	"regexp"
	"strings"
)

// portable env var names: what every shell can reference as $NAME
var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateName is the one place that decides which names steps and configs may set
// even with trusted authors this catches mistakes early: a step that overwrote
// QAHYA_JOB_API would cut every later step off from the runner
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%q is not a valid variable name", name)
	}
	if name == "PATH" {
		// the runner prepends its own dir so steps can find `qahyacli`;
		// PATH edits get their own prepend/append commands later
		return fmt.Errorf("PATH is managed by the runner")
	}
	if strings.HasPrefix(name, "QAHYA_") {
		return fmt.Errorf("%s: QAHYA_ prefix is reserved for the runner", name)
	}
	return nil
}

