package grant

import (
	"context"
	"fmt"
	"regexp"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/mq"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

// Backend identifies which transport a grant resolves to.
type Backend int

const (
	BackendDB Backend = iota
	BackendMQ
)

func (b Backend) String() string {
	if b == BackendMQ {
		return "mq"
	}
	return "db"
}

// Verb names a grant sub-command.
type Verb string

const (
	VerbCurrency    Verb = "currency"
	VerbItem        Verb = "item"
	VerbSkillpoints Verb = "skillpoints"
)

const (
	maxSkillpoints = 1000
	maxItemCount   = 1000
	maxCurrency    = 1_000_000_000
)

var itemNameRE = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

// Req describes one requested grant.
type Req struct {
	Verb       Verb
	FLS        string
	CurrencyID int
	Delta      int64
	Item       string
	Count      int64
	Quality    int64
	Durability float64
	Amount     int64
	Force      bool
}

// Plan is the resolved, not-yet-applied action (the --check view).
type Plan struct {
	Verb    Verb
	FLS     string
	Offline bool
	Backend Backend
	Summary string
}

// Result is the outcome of Apply.
type Result struct {
	OK     bool
	Detail string
}

// Granter orchestrates presence resolution + backend routing + audit.
type Granter struct {
	DB    *dbquery.Runner
	MQ    *mq.Publisher
	Store *store.Store
}

func (g *Granter) validate(req Req) error {
	if req.FLS == "" {
		return fmt.Errorf("grant: empty fls")
	}
	switch req.Verb {
	case VerbCurrency:
		if req.Delta == 0 || req.Delta > maxCurrency || req.Delta < -maxCurrency {
			return fmt.Errorf("grant currency: delta %d out of range (±%d, non-zero)", req.Delta, maxCurrency)
		}
	case VerbItem:
		if !itemNameRE.MatchString(req.Item) {
			return fmt.Errorf("grant item: invalid item id %q", req.Item)
		}
		if req.Count <= 0 || req.Count > maxItemCount {
			return fmt.Errorf("grant item: count %d out of range (1..%d)", req.Count, maxItemCount)
		}
	case VerbSkillpoints:
		if req.Amount <= 0 || req.Amount > maxSkillpoints {
			return fmt.Errorf("grant skillpoints: amount %d out of range (1..%d)", req.Amount, maxSkillpoints)
		}
	default:
		return fmt.Errorf("grant: unknown verb %q", req.Verb)
	}
	return nil
}

// Plan validates, resolves presence, and chooses a backend without writing.
func (g *Granter) Plan(ctx context.Context, host string, req Req) (Plan, error) {
	if err := g.validate(req); err != nil {
		return Plan{}, err
	}
	offline, err := g.DB.IsPlayerOffline(ctx, host, req.FLS)
	if err != nil {
		return Plan{}, fmt.Errorf("grant: presence check: %w", err)
	}
	p := Plan{Verb: req.Verb, FLS: req.FLS, Offline: offline}
	switch req.Verb {
	case VerbItem:
		if offline {
			p.Backend = BackendDB
			p.Summary = fmt.Sprintf("insert %d×%s into backpack (offline, DB)", req.Count, req.Item)
		} else {
			p.Backend = BackendMQ
			p.Summary = fmt.Sprintf("AddItemToInventory %d×%s (online, MQ)", req.Count, req.Item)
		}
	case VerbCurrency:
		p.Backend = BackendDB
		if !offline && !req.Force {
			return Plan{}, fmt.Errorf("grant currency: player online; the running game owns the balance and may overwrite a DB write — re-run with --force to apply anyway")
		}
		p.Summary = fmt.Sprintf("currency %d delta %+d (DB)", req.CurrencyID, req.Delta)
	case VerbSkillpoints:
		p.Backend = BackendDB
		if !offline && !req.Force {
			return Plan{}, fmt.Errorf("grant skillpoints: player online; the running game owns character state and may overwrite a DB write — re-run with --force to apply anyway")
		}
		p.Summary = fmt.Sprintf("skillpoints +%d (DB)", req.Amount)
	}
	return p, nil
}

// Apply re-plans (re-validate + re-resolve presence) then executes + audits.
func (g *Granter) Apply(ctx context.Context, operator, host string, req Req) (Result, error) {
	p, err := g.Plan(ctx, host, req)
	if err != nil {
		return Result{}, err
	}
	switch req.Verb {
	case VerbCurrency:
		bal, err := g.DB.GrantCurrency(ctx, host, req.FLS, req.CurrencyID, req.Delta)
		g.audit(operator, host, "give.currency",
			fmt.Sprintf("fls=%s currency=%d delta=%d backend=db", req.FLS, req.CurrencyID, req.Delta), err)
		if err != nil {
			return Result{}, err
		}
		return Result{OK: true, Detail: fmt.Sprintf("new balance %d", bal)}, nil
	case VerbSkillpoints:
		unspent, err := g.DB.GrantSkillpoints(ctx, host, req.FLS, req.Amount)
		g.audit(operator, host, "give.skillpoints",
			fmt.Sprintf("fls=%s amount=%d backend=db", req.FLS, req.Amount), err)
		if err != nil {
			return Result{}, err
		}
		return Result{OK: true, Detail: fmt.Sprintf("unspent now %d", unspent)}, nil
	case VerbItem:
		if p.Backend == BackendDB {
			id, err := g.DB.GrantItemDB(ctx, host, req.FLS, req.Item, req.Count, req.Quality)
			g.audit(operator, host, "give.item",
				fmt.Sprintf("fls=%s item=%s count=%d backend=db", req.FLS, req.Item, req.Count), err)
			if err != nil {
				return Result{}, err
			}
			return Result{OK: true, Detail: fmt.Sprintf("item id %d", id)}, nil
		}
		dur := req.Durability
		if dur == 0 {
			dur = 1.0
		}
		inner := mq.BuildAddItemCommand(req.FLS, req.Item, req.Count, dur)
		subject := fmt.Sprintf("fls=%s item=%s count=%d backend=mq", req.FLS, req.Item, req.Count)
		// PublishInner records its own audit entry (action "give.item").
		res, err := g.MQ.PublishInner(ctx, operator, host, "give.item", subject, inner, "additem")
		if err != nil {
			return Result{}, err
		}
		return Result{OK: res.OK, Detail: res.RawOutput}, nil
	}
	return Result{}, fmt.Errorf("grant: unknown verb %q", req.Verb)
}

func (g *Granter) audit(operator, host, action, subject string, opErr error) {
	e := store.AuditEntry{Operator: operator, Host: host, Action: action, Subject: subject, Result: "ok"}
	if opErr != nil {
		e.Result = "error: " + opErr.Error()
	}
	_ = g.Store.AppendAudit(e)
}
