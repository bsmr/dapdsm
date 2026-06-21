package worldsetup

import "strings"

// PlaceholderImageTag is the tag world.sh's templates carry ({WORLD_IMAGE_TAG});
// the real depot revision is reconciled onto the CR afterwards (see
// battlegroup.BuildImageTagPatches).
const PlaceholderImageTag = "0-0-shipping"

// yamlQuote renders s as a YAML double-quoted scalar, escaping backslash and
// double-quote, so any printable BattleGroup title (spaces, colons, slashes,
// dots) is safe in the Funcom template's bare `title: {WORLD_NAME}` slot.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// renderTemplate substitutes each {KEY} with vals[KEY]. Values are inserted
// verbatim (base64 secrets may contain '/' and '='); the Funcom templates quote
// the fields that need quoting.
func renderTemplate(tmpl string, vals map[string]string) string {
	pairs := make([]string, 0, len(vals)*2)
	for k, v := range vals {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
