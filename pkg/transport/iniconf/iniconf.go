// Package iniconf edits Funcom-style INI files (UserEngine.ini,
// UserGame.ini) in-place: setting a Section.Key to a value, uncommenting
// existing entries, and appending missing sections / keys when needed.
//
// The format is the Unreal-Engine INI dialect:
//   - sections look like [Name] or [/Script/Module.Class]
//   - keys look like Name=Value (no spaces around =)
//   - lines starting with ';' are comments; "uncommenting" simply means
//     dropping the leading ';' and rewriting the key=value line
//   - quoting is operator-side: this package neither adds nor removes
//     quotes; NeedsQuoting / Quote help the CLI decide when to wrap
//
// The package is intentionally line-oriented (no full AST). Funcom INIs
// are tiny enough that this is faster to reason about than a full parser
// and preserves operator comments byte-for-byte.
package iniconf

import (
	"fmt"
	"strconv"
	"strings"
)

// SetKey returns content with `[section]` updated so that key=value.
//
// Behaviour:
//   - If section already contains a live "Key=..." line, replace it.
//   - If section contains ";Key=..." (commented), drop the ';' and rewrite.
//   - If section exists but does not mention key, append a new line at
//     the end of the section, before any trailing blank lines.
//   - If section does not exist, append a fresh "[section]\nkey=value\n"
//     block at the end of the file.
//
// value is written verbatim — quote at the call site if you need it.
func SetKey(content []byte, section, key, value string) ([]byte, error) {
	if section == "" {
		return nil, fmt.Errorf("section must not be empty")
	}
	if key == "" {
		return nil, fmt.Errorf("key must not be empty")
	}

	lines := strings.Split(string(content), "\n")
	sectionHeader := "[" + section + "]"
	newLine := key + "=" + value

	// Locate the target section and any existing entry for key inside it.
	sectionStart := -1
	sectionEnd := -1
	keyLine := -1
	matches := matcherFor(key)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			if sectionStart == -1 {
				if trimmed == sectionHeader {
					sectionStart = i
				}
				continue
			}
			if sectionEnd == -1 {
				sectionEnd = i
				break
			}
		}
		if sectionStart != -1 && sectionEnd == -1 && matches(trimmed) {
			keyLine = i
		}
	}
	if sectionStart != -1 && sectionEnd == -1 {
		sectionEnd = len(lines)
	}

	if keyLine != -1 {
		// Replace the entry — works for both live and commented lines.
		lines[keyLine] = newLine
		return []byte(strings.Join(lines, "\n")), nil
	}

	if sectionStart == -1 {
		// Section absent: append at EOF.
		var b strings.Builder
		b.Write(content)
		if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
			b.WriteByte('\n')
		}
		if len(content) > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(sectionHeader)
		b.WriteByte('\n')
		b.WriteString(newLine)
		b.WriteByte('\n')
		return []byte(b.String()), nil
	}

	// Section present but key missing: insert before the trailing blanks
	// of the section so the file stays cosmetically tidy.
	insertAt := sectionEnd
	for insertAt > sectionStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, newLine)
	out = append(out, lines[insertAt:]...)
	return []byte(strings.Join(out, "\n")), nil
}

func isSectionHeader(trimmed string) bool {
	return strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && len(trimmed) > 2
}

// matcherFor returns a predicate that recognises any line "Key=..." or
// ";Key=..." (with optional whitespace) targeting key. Leading '+' from
// UE array-add syntax (";+m_PvpEnabledPartitions=1") is also stripped.
func matcherFor(key string) func(trimmedLine string) bool {
	return func(s string) bool {
		for strings.HasPrefix(s, ";") {
			s = strings.TrimSpace(s[1:])
		}
		s = strings.TrimPrefix(s, "+")
		name, _, ok := strings.Cut(s, "=")
		if !ok {
			return false
		}
		return strings.TrimSpace(name) == key
	}
}

// NeedsQuoting returns true if value looks like a plain string and so
// should be wrapped in double quotes for the Funcom INI format. Bool
// literals, integers, floats, and already-quoted strings pass through
// unchanged.
func NeedsQuoting(value string) bool {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return false
	}
	switch value {
	case "true", "false", "True", "False":
		return false
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return false
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return false
	}
	return true
}

// Quote wraps value in double quotes.
func Quote(value string) string {
	return `"` + value + `"`
}
