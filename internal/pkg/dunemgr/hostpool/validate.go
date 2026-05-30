// Package hostpool registers and tracks operator-defined hosts. It wraps
// the store with input validation; hosts are reached over SSH, so no
// cluster bootstrap (CA fetch) happens here.
package hostpool

import (
	"fmt"
	"regexp"
	"strings"
)

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)

// ValidateName checks that name is a safe host-profile key.
// Constraints: 1..63 chars, [A-Za-z0-9_-], no leading/trailing
// hyphen, no spaces or slashes.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name: empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("name: too long (%d > 63)", len(name))
	}
	if !nameRE.MatchString(name) {
		return fmt.Errorf("name %q: invalid (allowed [A-Za-z0-9_-], no leading/trailing -)", name)
	}
	return nil
}

// ValidateSSHAlias rejects empty strings, flag-prefixed values
// (defense-in-depth on top of ssh package's mustNotBeFlag), and
// whitespace.
func ValidateSSHAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("ssh alias: empty")
	}
	if strings.HasPrefix(alias, "-") {
		return fmt.Errorf("ssh alias %q: cannot start with '-' (flag-smuggling guard)", alias)
	}
	if strings.ContainsAny(alias, " \t\n") {
		return fmt.Errorf("ssh alias %q: whitespace not allowed", alias)
	}
	return nil
}
