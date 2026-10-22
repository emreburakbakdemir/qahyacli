// package shell produces text that a POSIX shell will read back exactly.
package shell

import "strings"

// Quote makes s a single shell word. Inside single quotes nothing is special
// except ' itself, so each ' closes the quote, adds an escaped ', and reopens:
//
//	it's $(whoami) -> 'it'\''s $(whoami)'
//
// Newlines are fine inside single quotes. Without this, a value containing
// $(...) would be executed by `eval`.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
