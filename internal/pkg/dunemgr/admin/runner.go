package admin

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/mq"
)

// publisher is a narrow interface around mq.Publisher so Runner can be tested
// against a recorder without a real SSH connection.
type publisher interface {
	PublishInner(ctx context.Context, operator, host, action, subject, innerJSON, label string) (*mq.Result, error)
}

// Runner executes admin verbs against a game server via the MQ transport.
// MQ must be non-nil at call time.
type Runner struct {
	MQ publisher
}

// Run gates, builds, and publishes one admin command.
//
// verb     — registered admin verb (see Verbs()).
// playerID — FLS player ID, or "*" for all online players (subject to allowAll).
// args     — map of JSON field names → string values (flag parsing is the caller's job).
// confirm  — must be true for destructive verbs and for playerID="*".
func (r *Runner) Run(
	ctx context.Context,
	operator, host, verb, playerID string,
	args map[string]string,
	confirm bool,
) (*mq.Result, error) {
	s, ok := specFor(verb)
	if !ok {
		return nil, fmt.Errorf("admin: unknown verb %q", verb)
	}

	if s.destructive && !confirm {
		return nil, fmt.Errorf("admin %s: verb is destructive; pass --confirm", verb)
	}

	if playerID == "*" {
		if !s.allowAll {
			return nil, fmt.Errorf("admin %s: verb does not support wildcard player (*)", verb)
		}
		if !confirm {
			return nil, fmt.Errorf("admin %s: wildcard player (*) is a mass action; pass --confirm", verb)
		}
	}

	inner, err := Build(verb, playerID, args)
	if err != nil {
		return nil, err
	}

	subject := fmt.Sprintf("host=%s player=%s", host, playerID)
	return r.MQ.PublishInner(ctx, operator, host, "admin."+verb, subject, inner, verb)
}
