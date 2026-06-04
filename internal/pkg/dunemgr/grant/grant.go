package grant

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

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
	VerbXP          Verb = "xp"
	VerbCharXP      Verb = "charxp"
)

const (
	maxSkillpoints = 1000
	// maxItemCount is a self-imposed blast-radius guard on `give item`, NOT a
	// game/DB limit: the Funcom DB enforces no per-template stack max (verified
	// live — dune.items has no template-max column, and merge_inventory_items /
	// save_item never clamp; the real max lives in game assets and may be raised
	// by a server stack-size multiplier). Set generously to cover known high
	// stacks (e.g. SolarisCoin/"Solari" ~50000). Provisional/tunable pending the
	// online test of what the game server does with an over-max stack on load.
	maxItemCount = 50000
	maxCurrency  = 1_000_000_000
	maxTrackXP   = 44182
	maxCharXP    = 344440
)

// validTracks is the allowlist of dune.specializationtracktype values give-xp
// accepts. Only "Combat" is live-proven (admin xp hardcodes it); the rest are
// inferred from the enum and accepted on the operator's judgement. Source:
// submodules/dune-admin enum usage @4e09a92.
var validTracks = []string{
	"BeneGesserit", "Combat", "Melee", "Mentat",
	"Planetologist", "Swordmaster", "Trooper", "Vehicle",
}

// Tracks returns the sorted track allowlist (for usage text).
func Tracks() []string {
	out := append([]string(nil), validTracks...)
	sort.Strings(out)
	return out
}

// CanonicalTrack maps a case-insensitive track name to its canonical form.
func CanonicalTrack(s string) (string, bool) {
	for _, t := range validTracks {
		if strings.EqualFold(t, s) {
			return t, true
		}
	}
	return "", false
}

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
	Track      string // VerbXP
	XP         int64  // VerbXP, VerbCharXP
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
	case VerbXP:
		if _, ok := CanonicalTrack(req.Track); !ok {
			return fmt.Errorf("grant xp: unknown track %q (want one of %v)", req.Track, Tracks())
		}
		if req.XP <= 0 || req.XP > maxTrackXP {
			return fmt.Errorf("grant xp: amount %d out of range (1..%d)", req.XP, maxTrackXP)
		}
	case VerbCharXP:
		if req.XP <= 0 || req.XP > maxCharXP {
			return fmt.Errorf("grant charxp: amount %d out of range (1..%d)", req.XP, maxCharXP)
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
		if offline {
			p.Backend = BackendDB
			p.Summary = fmt.Sprintf("skillpoints +%d → Unspent (offline, DB)", req.Amount)
		} else {
			if !req.Force {
				return Plan{}, fmt.Errorf("grant skillpoints: player online; the online path sets Unspent from a possibly-stale DB read — re-run with --force to apply anyway")
			}
			p.Backend = BackendMQ
			p.Summary = fmt.Sprintf("skillpoints +%d → set Unspent base+%d (online, MQ; base may be stale)", req.Amount, req.Amount)
		}
	case VerbXP:
		if offline {
			p.Backend = BackendDB
			p.Summary = fmt.Sprintf("track xp +%d → %s (offline, DB)", req.XP, req.Track)
		} else {
			p.Backend = BackendMQ
			p.Summary = fmt.Sprintf("AwardXP +%d → %s (online, MQ)", req.XP, req.Track)
		}
	case VerbCharXP:
		p.Backend = BackendDB
		if !offline && !req.Force {
			return Plan{}, fmt.Errorf("grant charxp: character XP has no live path; player online — re-run with --force to write the DB anyway")
		}
		p.Summary = fmt.Sprintf("char xp +%d → recompute level/SP/intel (offline, DB)", req.XP)
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
		if p.Backend == BackendDB {
			unspent, err := g.DB.GrantSkillpoints(ctx, host, req.FLS, req.Amount)
			g.audit(operator, host, "give.skillpoints",
				fmt.Sprintf("fls=%s amount=%d backend=db", req.FLS, req.Amount), err)
			if err != nil {
				return Result{}, err
			}
			return Result{OK: true, Detail: fmt.Sprintf("unspent now %d", unspent)}, nil
		}
		base, err := g.DB.UnspentSkillpoints(ctx, host, req.FLS)
		if err != nil {
			return Result{}, err
		}
		target := base + req.Amount
		inner := mq.BuildSkillpointsCommand(req.FLS, target)
		subject := fmt.Sprintf("fls=%s amount=%d base=%d set=%d backend=mq", req.FLS, req.Amount, base, target)
		res, err := g.MQ.PublishInner(ctx, operator, host, "give.skillpoints", subject, inner, "skillpoints")
		if err != nil {
			return Result{}, err
		}
		return Result{OK: res.OK, Detail: fmt.Sprintf("set unspent %d (base %d + %d; base may be stale)", target, base, req.Amount)}, nil
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
	case VerbXP:
		if p.Backend == BackendDB {
			n, err := g.DB.GrantTrackXP(ctx, host, req.FLS, req.Track, req.XP)
			g.audit(operator, host, "give.xp",
				fmt.Sprintf("fls=%s track=%s amount=%d backend=db", req.FLS, req.Track, req.XP), err)
			if err != nil {
				return Result{}, err
			}
			return Result{OK: true, Detail: fmt.Sprintf("%s xp now %d", req.Track, n)}, nil
		}
		inner := mq.BuildAwardXPCommand(req.FLS, req.Track, req.XP)
		subject := fmt.Sprintf("fls=%s track=%s amount=%d backend=mq", req.FLS, req.Track, req.XP)
		res, err := g.MQ.PublishInner(ctx, operator, host, "give.xp", subject, inner, "awardxp")
		if err != nil {
			return Result{}, err
		}
		return Result{OK: res.OK, Detail: res.RawOutput}, nil
	case VerbCharXP:
		out, err := g.DB.GrantCharXP(ctx, host, req.FLS, req.XP)
		g.audit(operator, host, "give.charxp",
			fmt.Sprintf("fls=%s amount=%d backend=db", req.FLS, req.XP), err)
		if err != nil {
			return Result{}, err
		}
		capped := ""
		if out.Capped {
			capped = " (capped)"
		}
		return Result{OK: true, Detail: fmt.Sprintf("level %d, unspent %d%s", out.NewLevel, out.NewUnspent, capped)}, nil
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
