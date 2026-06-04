# dunemgr TUI Operator Guide

As of 0.1.12, the TUI is `dunemgr`'s default interface.
The web UI is disabled: the `serve`, `--print-token`, and `regen-token`
verbs are not wired and are rejected as unknown.

## Launch

```sh
dunemgr          # no-args default → TUI
dunemgr tui      # explicit
```

A real TTY is required.
Over SSH, use `ssh -t <host>` (or ensure your SSH config sets `RequestTTY yes`).
A no-TTY run exits with an error (bubbletea cannot start without a TTY).

## Layout

Three full-width sections stacked vertically, each drawn with a rounded border
and the section title embedded in the top edge (`╭─ Hosts ───╮`):

**Hosts** — all configured hosts on one line.
The active host is marked `▸`; each badge shows `●/○` (reachable),
BattleGroup state (`RUNNING`, `DEGRADED`, …), and ready/total pod count.
Host status lives only here.

**Content** — fills the remaining terminal height between the header and footer
(the footer is pinned to the bottom).
Title = breadcrumb + mode tag + position:
`vm-dune-01 › CharName › Backpack  [NAV] ITEM  3/12`.
In nav mode it shows the list for the current drill level.
After a `:` command it shows the command output; the nav list returns on `←`/`Esc`.

**Keys / Command** (footer) — pinned to the bottom.
Title is **Keys** in nav mode and **Command** (accent-coloured) in `:` mode.
In nav mode: a two-line key legend (level-specific keys, then
`[:] command   [?] help   [q] quit`).
In command mode: the text-input bar (`[vm-dune-01] › `) plus a multi-line helper
below it (see [Command mode helper](#command-mode-helper)).
While a command is in-flight a spinner (`⣾ running…`) appends below the footer.

## Modes

Two modes: **nav** (default, vi-style modal drill) and **cmd** (`:` command bar).

The content-box title reflects the current mode: `[NAV]` or `[CMD]` appears
before the level label.
In command mode the footer title is accent-highlighted as a subtle visual cue.

### Nav mode — key table

| Key | Action |
|---|---|
| `↑` / `k` | Move cursor up one row |
| `↓` / `j` | Move cursor down one row |
| `→` / `Enter` / `l` | Descend one level (Hosts → Players → Inventory → Item) |
| `←` / `Esc` / `h` | Ascend one level (or dismiss command output) |
| `PgUp` | Move cursor up one page |
| `PgDn` | Move cursor down one page |
| `g` | Jump to first row |
| `G` | Jump to last row |
| `a` | Add item — pre-fills `give item <player> ` in the command bar (all levels with a player in context) |
| `s` | Cycle sort column (PLAYERS and ITEM levels; skips non-sortable columns) |
| `S` | Toggle sort direction ascending ↔ descending (PLAYERS and ITEM levels) |
| `?` | Open the help overlay |
| `:` | Enter command mode |
| `q` / `Ctrl-C` | Quit |

**Sortable lists.** The PLAYERS and ITEM nav lists support interactive sorting.
`s` cycles through the sortable columns; `S` toggles ascending/descending.
The active sort column is marked `^` (ascending) or `v` (descending) in the pinned header.
Sorting at non-table levels (HOSTS, INVENTORY) is a no-op.

**`?` help overlay.** Pressing `?` in nav mode replaces the content box with a
full key/mode reference — the primary discoverability aid.
Any key (including `Esc`) dismisses the overlay and returns to the nav list.
The overlay cannot open while an inline edit, confirm-delete, or command result
is active.

**Viewing command output:** when a `:` command produced output, the content box
shows it. `↑`/`↓`/`k`/`j`/`PgUp`/`PgDn` scroll the output; `←`/`Esc`/`h`
dismisses it back to the nav list.
While output is showing, the footer displays `←/Esc back   ↑↓ scroll` as a
visible dismiss hint instead of the normal key legend.

### Item-level edit keys (nav mode, level ITEM only)

| Key | Action |
|---|---|
| `+` | Stack size +1 (capped at stack max if known; shows "already at max stack (N)" otherwise) |
| `-` | Stack size −1 (floor 0) |
| `e` | Type a new stack size (Enter applies, Esc cancels; rejects values above stack max: "max stack is N") |
| `Q` | Type a new quality (Enter applies, Esc cancels; see note below) |
| `d` | Delete item — prompts `y/n`; any other key cancels |
| `a` | Pre-fill command bar with `give item <player> ` (Tab-completes item template, then type count) |

**Stack limits.** When the vendored catalog knows the max stack for a template, the MAX column
shows the limit. `+` is blocked at max and shows "already at max stack (N)"; `e` rejects
values above max with "max stack is N". Non-stackable items (max 1) cannot be incremented.

**Quality note.** `Q` edits `quality_level`.
The field is rarely populated in practice and its effect is undocumented — no fixed
range is known. A warning "quality_level is rarely used; effect undocumented" appears
inline when the prompt opens.

Edits are refused if the item's owner is online or unresolvable;
the content box shows the reason.
`--force` is available only on the CLI `item` verb — there is no TUI override.

### Command mode — key table

| Key | Action |
|---|---|
| `Tab` | Complete current token |
| `Enter` | Run command |
| `Esc` | Cancel, return to nav mode |
| `↑` | Recall previous history entry |
| `↓` | Recall next history entry (or clear) |

## Drill Levels

The modal nav descends through four levels:

| Level | Content |
|---|---|
| **HOSTS** | All configured hosts |
| **PLAYERS** | Everyone who has joined that host (character name + online status) |
| **INVENTORY** | The player's inventories by human label (`Backpack`, `Equipment`, …) and item count (loaded in a single query) |
| **ITEM** | Items of that inventory: `id=<N>  <display name>  <stack>  <max>  q<quality>` (STACK and MAX are separate right-aligned columns; MAX blank when not in catalog) |

Switching levels shows `loading…` in the content box until the data arrives
— stale rows from the previous level are never shown during the SSH round-trip.

The breadcrumb in the content-box title tracks the selection:
`vm-dune-01 › CharName › Backpack  [NAV] ITEM  3/17`.

### Item display names

Item rows show a human display name sourced from the vendored catalog
(e.g. `Ornate Hookah Pipe` instead of a raw template id).
Uncatalogued template ids fall back to the raw id unchanged.
The `dune.items.id` (edit handle), stack, and quality are always shown.
STACK and MAX are separate right-aligned columns; MAX is blank for templates not in the catalog.

The same name resolution applies to the CLI `player … inspect --inv <type>` listing.

## Pinned Column Headers

Every nav drill level except HOSTS shows a **pinned column header row** above the
scrolling data list.
The header stays fixed while the operator pages through long lists; only the data
rows beneath it scroll.

| Level | Pinned header |
|---|---|
| PLAYERS | `NAME                 STATUS` |
| INVENTORY | `TYPE         ITEMS` |
| ITEM | `ID  NAME  STACK  MAX  Q` (column widths auto-sized to content; STACK and MAX are separate right-aligned columns) |

Command output from tabular verbs (`player search`, `avatar list`, `avatar exports`)
also pins its first line(s) as a fixed header: the column labels stay visible while
the data rows scroll.

## Command Mode Helper

While the `:` command bar is open, a helper area of up to four lines appears
below the input.
It shows one of two things:

- **Completion candidates** (when Tab would offer completions) — laid out as a
  grid of uniform-width cells, wrapping across rows.
  Example after typing `:avatar `:

  ```
  export  list  exports  import  transfer
  ```

- **Usage grammar** (when the current input matches a known verb but there are
  no completions to cycle) — the full flag-bearing synopsis, word-wrapped to the
  terminal width.
  Example after typing `:item `:

  ```
  item <host> <set|delete> <item-id>
  | [--qty N] [--quality Q] [--confirm] [--force]
  ```

When there are more candidates or grammar lines than fit in four rows, a
`(+N more)` marker replaces the tail — the input field is never pushed off screen.

## Inventory Labels

Raw Funcom `inventory_type` values (live-confirmed):

| Type | Label |
|---|---|
| 0 | Backpack |
| 1 | Equipment |
| 14 | Emotes |
| 15 | Hotbar |
| 27 | Emotes (2) |
| other | `type N` |

## Command Bar

The prompt shows the selected host: `[vm-dune-01] › `.

**Implied host.** For all host-first verbs (`stats`, `lifecycle`, `player`, `give`,
`admin`, `whisper`, `avatar`, `ini`, `broadcast`, `backup`, `shutdown`, `db`, `item`),
the selected host is injected automatically on submit.
Type `whisper Sti⇥` — not `whisper vm-dune-01 Sti`.
The host never appears in the input field; it is added invisibly at dispatch time.

**Tab completion** works at every token position:

- First token: verb names.
- Sub-verb slot: fixed options (e.g. `start|stop|restart|update`).
- Host slot (when a verb takes two hosts, such as `avatar transfer`): known host names.
- Player-name slot (`player`, `give`, `admin`, `whisper`, `avatar`): live cache of
  character names fetched from the selected host's database.
- Catalog slot (`give item` template, `admin item`/`skill`/`vehicle` name): vendored item/skill/vehicle catalog.

**Player-name cache** is filled lazily on the first Tab press in a name slot.
It lists every FLS account that has ever joined the server, not just online players.
`:refresh` drops and re-fetches the cache for the selected host.

**Built-in verbs** (handled by the TUI, not the dispatcher):

| Built-in | Effect |
|---|---|
| `:help [verb]` | Show usage for a verb, or the full verb list |
| `:refresh` | Drop the player-name completion cache for the selected host |

## Per-Verb Grammar

The grammar column matches the usage strings in the source.
`<host>` is the selected host, injected on submit.

| Verb | Grammar |
|---|---|
| `host` | `host <add\|list\|probe\|rm> [name] [--ssh-alias=alias]` |
| `stats` | `stats <host>` |
| `lifecycle` | `lifecycle <host> <start\|stop\|restart\|update>` |
| `player` | `player <host> <search\|pos\|inspect> [<name\|fls>] [--inv <type>] [--top N] [--raw] [--id]` |
| `item` | `item <host> <set\|delete> <item-id> [--qty N] [--quality Q] [--confirm] [--force]` |
| `give` | `give <host> <currency\|item\|skillpoints\|xp\|charxp> <name\|fls> …` |
| `admin` | `admin <host> <verb> <name\|fls\|*> [args] [--id] [--confirm]` |
| `whisper` | `whisper <host> <name\|fls> <message> [--from <name>] [--force] [--as <GM\|Server>] [--id]` |
| `avatar` | `avatar <host> <export\|list\|exports\|import\|transfer> …` |
| `ini` | `ini <host> <list\|get\|set> [key] [value] [--apply] [--restart]` |
| `broadcast` | `broadcast <host> <notice\|shutdown\|shutdown-cancel> [flags]` |
| `backup` | `backup <host> <bg> <create\|list\|restore> …` |
| `shutdown` | `shutdown <host> <schedule\|cancel\|status> [flags]` |
| `db` | `db <host> <exec\|columns\|slow> …` |

For full flag details run `:help <verb>` in the TUI or pass `--help` on the CLI.

### avatar sub-verb detail

| Sub-verb | Extra args |
|---|---|
| `export` | `<name\|fls> [--id]` |
| `list` | _(no args — reads server)_ |
| `exports` | _(no args — reads local store)_ |
| `import` | `<name\|fls> <export-key> [--name <n>] [--id] --confirm` |
| `transfer` | `<dst-host> <name\|fls> [--name <n>] [--check] [--id] --confirm` |

### admin verbs

Available verbs (from the admin package): `item`, `skill`, `vehicle`,
`skillpoints`, `water`, `xp`, `kick`, `clean`, `reset`, `teleport`,
`teleport-exact`. Run `:help admin` for the full list at runtime.

### Skillpoints — two paths

| Path | Grammar | Semantics |
|---|---|---|
| `give skillpoints <player> N` | additive, presence-aware | **Recommended.** Adds N to the player's current unspent points via MQ; checks online status. |
| `admin skillpoints <player> --points N` | absolute MQ set | Raw override. Sets unspent points to exactly N. `--points` is **required** — omitting it returns a usage error and steers you to `give skillpoints`. |

Use `give skillpoints` for day-to-day grants.
Use `admin skillpoints --points` only when you need to set an exact value (e.g. zeroing points after a reset).

## Notes

- `player search` (and the name-completion cache) returns every FLS account
  that has ever joined the server, not just online players.
- `avatar list` reads the **server** (joinable characters currently registered);
  `avatar exports` lists locally-saved export records on this workstation.
- Export and backup keys use `/` as the delimiter (`host/fls-ts`, `host/bg/ts`).
  Host aliases never contain `/`, so keys are unambiguous and copy-pasteable
  directly into `avatar import` or `backup restore`.
- Destructive verbs (`admin kick`, `admin clean`, `admin reset`, `avatar import`,
  `avatar transfer`, `backup restore`) require `--confirm`.
- TUI item edits (at the ITEM drill level) are offline-gated: edits are refused
  if the owning player is online or cannot be resolved. `--force` is CLI-only
  (`item <host> set|delete <id> --force`); there is no TUI override.
- Item identity is the per-stack `dune.items.id`. Two stacks of the same template
  in different inventories have distinct ids — edits are never ambiguous.
- `give item` (including the `a` pre-fill shortcut) places the item in the player's
  **backpack** (inventory type 0). Equipment slots are not arbitrarily fillable.
- A host added via the command bar during a session streams live status only
  after a TUI restart; commands against it work immediately.
