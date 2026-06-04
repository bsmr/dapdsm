// Package command — item subcommand: edit/delete a single inventory item by id.
package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

const itemUsage = `usage:
  dunemgr item <host> set    <item-id> [--qty N] [--quality Q] [--confirm] [--force]
  dunemgr item <host> delete <item-id> [--confirm] [--force]

Edits target a single dune.items row by id (offline player only; --force to
override an online/unknown owner). Find ids via: dunemgr player <host> inspect <name> --inv <type>.`

// itemMutator is the dbquery surface the item-edit path needs; *dbquery.Runner
// satisfies it. Sharing this interface lets the CLI verb and the TUI use one
// gated apply path, and makes that path unit-testable with a fake.
type itemMutator interface {
	ItemOwnerFLS(ctx context.Context, host string, itemID int64) (string, error)
	IsPlayerOffline(ctx context.Context, host, fls string) (bool, error)
	SetItemStack(ctx context.Context, host string, itemID, stack int64) (int64, error)
	SetItemQuality(ctx context.Context, host string, itemID, quality int64) (int64, error)
	DeleteItem(ctx context.Context, host string, itemID int64) (int64, error)
}

var (
	// ErrItemOwnerOnline is returned by Apply* when the item's owner is online
	// and force is false.
	ErrItemOwnerOnline = errors.New("item owner is online")
	// ErrItemOwnerUnknown is returned by Apply* when the item's owner cannot be
	// resolved and force is false.
	ErrItemOwnerUnknown = errors.New("item has no resolvable player owner")
)

// itemGate checks that the item's owner is offline before a mutation is
// allowed. It is a no-op when force is true.
func itemGate(ctx context.Context, m itemMutator, host string, itemID int64, force bool) error {
	if force {
		return nil
	}
	owner, err := m.ItemOwnerFLS(ctx, host, itemID)
	if err != nil {
		return err
	}
	if owner == "" {
		return ErrItemOwnerUnknown
	}
	offline, err := m.IsPlayerOffline(ctx, host, owner)
	if err != nil {
		return err
	}
	if !offline {
		return ErrItemOwnerOnline
	}
	return nil
}

// ApplyItemStack gates then sets stack_size, auditing on success.
func ApplyItemStack(ctx context.Context, m itemMutator, s *store.Store, host string, itemID, stack int64, force bool) error {
	if err := itemGate(ctx, m, host, itemID, force); err != nil {
		return err
	}
	n, err := m.SetItemStack(ctx, host, itemID, stack)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no player item with id %d", itemID)
	}
	itemAudit(s, host, "item.set.qty", itemID, strconv.FormatInt(stack, 10))
	return nil
}

// ApplyItemQuality gates then sets quality_level, auditing on success.
func ApplyItemQuality(ctx context.Context, m itemMutator, s *store.Store, host string, itemID, quality int64, force bool) error {
	if err := itemGate(ctx, m, host, itemID, force); err != nil {
		return err
	}
	n, err := m.SetItemQuality(ctx, host, itemID, quality)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no player item with id %d", itemID)
	}
	itemAudit(s, host, "item.set.quality", itemID, strconv.FormatInt(quality, 10))
	return nil
}

// ApplyItemDelete gates then deletes, auditing on success.
func ApplyItemDelete(ctx context.Context, m itemMutator, s *store.Store, host string, itemID int64, force bool) error {
	if err := itemGate(ctx, m, host, itemID, force); err != nil {
		return err
	}
	n, err := m.DeleteItem(ctx, host, itemID)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no player item with id %d", itemID)
	}
	itemAudit(s, host, "item.delete", itemID, "")
	return nil
}

func itemCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 {
		fmt.Fprintln(stderr, itemUsage)
		return fmt.Errorf("item: usage: %w", ErrUsage)
	}
	host, sub := args[0], args[1]
	itemID, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		fmt.Fprintln(stderr, "item: <item-id> must be an integer")
		return fmt.Errorf("item: parse id: %w", ErrUsage)
	}
	fs := flag.NewFlagSet("item", flag.ContinueOnError)
	fs.SetOutput(stderr)
	qty := fs.Int64("qty", -1, "new stack size")
	quality := fs.Int64("quality", -1, "new quality level")
	confirm := fs.Bool("confirm", false, "confirm the write")
	force := fs.Bool("force", false, "apply even if the owning player is online / unknown")
	if err := fs.Parse(args[3:]); err != nil {
		return err
	}
	setQty, setQuality := false, false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "qty":
			setQty = true
		case "quality":
			setQuality = true
		}
	})

	applyErr := func(itemErr error, id int64) error {
		switch {
		case errors.Is(itemErr, ErrItemOwnerOnline):
			fmt.Fprintf(stderr, "item: owner of id %d is online (use --force)\n", id)
			return fmt.Errorf("item: owner online: %w", ErrUsage)
		case errors.Is(itemErr, ErrItemOwnerUnknown):
			fmt.Fprintf(stderr, "item: no resolvable player owner for id %d (use --force)\n", id)
			return fmt.Errorf("item: unknown owner: %w", ErrUsage)
		default:
			return itemErr
		}
	}

	switch sub {
	case "set":
		if !setQty && !setQuality {
			fmt.Fprintln(stderr, "item set: at least one of --qty / --quality is required")
			return fmt.Errorf("item set: no field: %w", ErrUsage)
		}
		if (setQty && *qty < 0) || (setQuality && *quality < 0) {
			fmt.Fprintln(stderr, "item set: --qty and --quality must be >= 0")
			return fmt.Errorf("item set: negative value: %w", ErrUsage)
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		if !*confirm {
			fmt.Fprintf(stdout, "item set: dry-run — would set")
			if setQty {
				fmt.Fprintf(stdout, " qty=%d", *qty)
			}
			if setQuality {
				fmt.Fprintf(stdout, " quality=%d", *quality)
			}
			fmt.Fprintln(stdout, " (pass --confirm to apply)")
			return nil
		}
		if setQty {
			if err := ApplyItemStack(ctx, r, c.Store, host, itemID, *qty, *force); err != nil {
				return applyErr(err, itemID)
			}
		}
		if setQuality {
			if err := ApplyItemQuality(ctx, r, c.Store, host, itemID, *quality, *force); err != nil {
				return applyErr(err, itemID)
			}
		}
		fmt.Fprintf(stdout, "item %d updated\n", itemID)
		return nil
	case "delete":
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		if !*confirm {
			fmt.Fprintln(stdout, "item delete: dry-run (pass --confirm to apply)")
			return nil
		}
		if err := ApplyItemDelete(ctx, r, c.Store, host, itemID, *force); err != nil {
			return applyErr(err, itemID)
		}
		fmt.Fprintf(stdout, "item %d deleted\n", itemID)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown item subcommand %q (want set|delete)\n", sub)
		return fmt.Errorf("item: unknown sub %q: %w", sub, ErrUsage)
	}
}

// itemAudit writes a no-PII audit entry (item id + field + new value only).
func itemAudit(s *store.Store, host, action string, itemID int64, value string) {
	if s == nil {
		return
	}
	subject := fmt.Sprintf("item=%d", itemID)
	if value != "" {
		subject += " value=" + value
	}
	_ = s.AppendAudit(store.AuditEntry{Operator: "cli", Host: host, Action: action, Subject: subject, Result: "ok"})
}
