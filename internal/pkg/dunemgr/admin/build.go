package admin

import (
	"encoding/json"
	"fmt"
	"strconv"

	admincatalog "go.muehmer.eu/dapdsm/pkg/domain/catalog"
)

// Build constructs the inner-JSON string for a ServerCommand payload.
//
// verb     — one of the registered verbs (Verbs()).
// playerID — the FLS player ID to inject as PlayerId; "*" is accepted here
//
//	(gating is the Runner's responsibility).
//
// args     — map of JSON field names to string values; unknown keys are silently
//
//	ignored. Numeric fields are coerced from string by kind.
//
// Returns an error when verb is unknown, a required field is absent, or a
// numeric coercion fails.
func Build(verb, playerID string, args map[string]string) (string, error) {
	s, ok := specFor(verb)
	if !ok {
		return "", fmt.Errorf("admin: unknown verb %q", verb)
	}

	payload := map[string]any{
		"ServerCommand": s.command,
		"PlayerId":      playerID,
	}

	for _, f := range s.fields {
		raw, present := args[f.json]
		if present && raw != "" {
			coerced, err := coerce(f.kind, raw)
			if err != nil {
				return "", fmt.Errorf("%s: field %s: %w", verb, f.json, err)
			}
			payload[f.json] = coerced
			continue
		}
		// Field absent or empty.
		if f.def != nil {
			payload[f.json] = f.def
			continue
		}
		if f.required {
			return "", fmt.Errorf("%s: %s required", verb, f.json)
		}
		// Optional with no default → omit (do not set null).
	}

	// Catalog validation: check catalog-backed fields against the embedded data.
	if err := validateCatalogFields(verb, payload); err != nil {
		return "", err
	}

	// Inject literals (always override, so inject after field loop).
	for k, v := range s.inject {
		payload[k] = v
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("admin: marshal %s payload: %w", verb, err)
	}
	return string(b), nil
}

// validateCatalogFields checks catalog-backed payload fields for verbs that
// carry an ItemName, Module(+Level), or ClassName(+TemplateName).
// payload is the partially built payload map (fields already coerced).
func validateCatalogFields(verb string, payload map[string]any) error {
	switch verb {
	case "item":
		name, _ := payload["ItemName"].(string)
		if name == "" {
			return nil // required-field check already handled above
		}
		ids := admincatalog.ItemIDs()
		for _, id := range ids {
			if id == name {
				return nil
			}
		}
		return fmt.Errorf("item: unknown item %q", name)

	case "skill":
		mod, _ := payload["Module"].(string)
		if mod == "" {
			return nil // required-field check already handled above
		}
		maxLevel, ok := admincatalog.SkillMaxLevel(mod)
		if !ok {
			return fmt.Errorf("skill: unknown skill module %q", mod)
		}
		// Level may be an int (coerced) or absent (default applied as int 1).
		if lvl, ok := payload["Level"].(int); ok && lvl > maxLevel {
			return fmt.Errorf("skill: level %d exceeds max level %d for module %q", lvl, maxLevel, mod)
		}

	case "vehicle":
		cls, _ := payload["ClassName"].(string)
		if cls == "" {
			return nil
		}
		templates, ok := admincatalog.VehicleTemplates(cls)
		if !ok {
			return fmt.Errorf("vehicle: unknown vehicle %q", cls)
		}
		tmpl, _ := payload["TemplateName"].(string)
		if tmpl == "" {
			return nil
		}
		for _, t := range templates {
			if t == tmpl {
				return nil
			}
		}
		return fmt.Errorf("vehicle: unknown template %q for vehicle %q (valid: %v)", tmpl, cls, templates)
	}
	return nil
}

// coerce converts a string value to the appropriate Go type for JSON encoding.
func coerce(kind fieldKind, raw string) (any, error) {
	switch kind {
	case kindString:
		return raw, nil
	case kindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q: %w", raw, err)
		}
		return n, nil
	case kindFloat:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("expected float, got %q: %w", raw, err)
		}
		return f, nil
	default:
		return raw, nil
	}
}
