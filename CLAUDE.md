# CLAUDE.md

## Language & Communication

- **Responses to the user**: Always in German — short, concise, technically precise.
- **Everything else** (code, commits, code comments, file contents): Always in English.
- **Prompt correction**: Check the user's German prompt for style, grammar, and spelling before answering. If errors are found: prepend brief corrections, then the actual answer.
- **Style (all languages)**: Precise, concise, technical. No filler text, no platitudes.

## Project Overview

**dapdsm** — _Dune: Awakening_ Private Dedicated Server Management.

Standalone home of the Go CLIs that drive private _Dune: Awakening_
dedicated servers running on a single-node Kubernetes cluster. The
binaries codify operator workflows that would otherwise be hand-rolled
against `kubectl`, the Funcom operator CRs, and Funcom's vendor
scripts.

Current binaries:

- **`ds-bashar`** — operator CLI: BattleGroup CR patches, Sietch
  scaling, INI editing, lifecycle (start/stop/restart/pre-shutdown),
  reconcile loop, Funcom `setup.sh` driver, post-update hook,
  player-IP apply, pod healing, broadcast messaging (broadcast +
  --announce).

Planned:

- **`dunemgr`** — design phase.
- Possibly more — see issues.

## Composition With the Meta-Repo

This repo is composed back into the
`meta-dune-awakening-private-dedicated-servers` repo as a git
submodule at `projects/dapdsm/`. The meta-repo provides operational
documentation, shell helpers, K3s installer drop-ins, deploy driver,
and pinned third-party reference repos under `submodules/`.

**Versions stay in lockstep with the meta-repo.** When the meta-repo
moves to a new `development-X.Y.Z-{main,work}` or
`production-X.Y.Z`, this repo follows on a matching branch. Submodule
pointer bumps in the meta-repo always target a dapdsm commit
reachable from the same version's branch here.

## Repository Layout

```text
.
├── cmd/<appname>/        # Go CLI entry points (one subdir per binary)
├── pkg/
│   ├── transport/        # Tool-agnostic mechanics (ssh, kube, iniconf, database, publicip), all kubectl-exec based.
│   ├── domain/           # Dune semantics composing transport (battlegroup, store, avatar, backup, broadcast,
│   │                     #   dbquery, gameini, grant, hostpool, lifecycle, mq, probe, stats, auth; market reserved).
│   │                     #   Tools call this layer.
│   └── version/          # Shared version info.
├── internal/
│   └── pkg/              # Tool-bound packages that remain private:
│       ├── dsbashar/     #   dsbashar/{cli,config}
│       └── dunemgr/      #   dunemgr/{cli,core,command,tui,ui,sse,server,admin,schedule,config}
├── etc/                  # Operator config templates (synced to /opt/dapdsm/etc on the host, served as /etc/dune/ samples)
│   └── dune/
├── scripts/              # Operator helpers (build, deploy)
├── bin/                  # Build output (gitignored)
├── go.mod
├── LICENSE
└── README.md
```

Directories appear once the corresponding content lands; do not
pre-create empty scaffolding.

### Standalone vs. Composed Use

`scripts/deploy.sh` is **standalone**: a fresh clone of this repo,
without the meta-repo, is enough to build ds-bashar and ship it to a
target VM. The meta-repo composes this repo as a submodule and
orchestrates host bootstrap (K3s installer, SteamCMD, Funcom operator
images) separately — host bootstrap is *not* a concern of this
repo.

## Build & Test Commands

- Build: `go build -o bin/<name> ./cmd/<name>` — never bare `go build`; output must land in `bin/`.
- Test: `go test ./...`
- Vet/lint: `go vet ./...`
- Format: `gofmt -w .` (or `goimports -w .` if available).

From the meta-repo root, the same commands work with `-C projects/dapdsm`:

```sh
go test -C projects/dapdsm ./...
go build -C projects/dapdsm -o ../../bin/ds-bashar ./cmd/ds-bashar
```

## Go Conventions

Mandatory `main()` → `run()` pattern for every Go binary in `cmd/`:

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %s\n", err)
        os.Exit(1)
    }
}

func run() error {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    return cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}
```

- `main()` only calls `run()` and handles `os.Exit` — never call `os.Exit` from `run()`.
- `run()` is **wiring only**: creates context, signal handling, delegates to `pkg/` or `internal/pkg/`.
- SDK logic lives in `pkg/transport/` and `pkg/domain/`; tool-bound logic stays in `internal/pkg/<name>/`; all I/O is injected (`context.Context`, `args []string`, `stdin io.Reader`, `stdout io.Writer`, `stderr io.Writer`).
- Every package has a `_test.go` with meaningful coverage. Write tests first, then implement.
- Build output always to `bin/`; `bin/` is gitignored.

## Access Model

Binaries that reach target VMs do so **exclusively via SSH**,
configured in `~/.ssh/config` and authenticated through `ssh-agent`
(with passphrase-protected keys held by the agent). Multi-hop access
uses `ProxyJump`. Target VM user is `dune` with passwordless `sudo`.

**Hard rule for Go code in this repo: never read SSH private keys
directly.** Acceptable patterns:

- `golang.org/x/crypto/ssh` with auth strictly via
  `golang.org/x/crypto/ssh/agent` over `SSH_AUTH_SOCK`.
- `os/exec` delegating to the system `ssh` binary, which resolves
  `~/.ssh/config`, `ProxyJump`, and `IdentityFile` itself.
- Parsing `~/.ssh/config` for host-alias resolution (e.g.
  `github.com/kevinburke/ssh_config`) is fine — provided
  `IdentityFile` is only **referenced**, never **opened**.

Forbidden in Go: `ssh.ParsePrivateKey` on file contents, reading
`~/.ssh/id_*` files, prompting for key passphrases inside the
program.

## Style Guide

- **Go**: [Google Go Style Guide](https://google.github.io/styleguide/go/)
- **Markdown**: ATX-style headings, fenced code blocks with language tags, one sentence per line is welcome for diff-friendliness but not required.

## Git Workflow

Branch model:

- `main` — current production commit, always deployable, protected.
- `production-X.Y.Z` — created from `main` after release.
- `development-X.Y.Z-{main,work}` — versioned development; `-main` tracks `main` state, `-work` is the workspace.
- `feature-<name>-{main,work}` / `fix-<name>-{main,work}` / `hotfix-<name>-{main,work}` — descriptive names, no version in the name.

Merge order (always sync `main` first via `git fetch --all && git checkout main && git pull --ff-only`):

1. Squash-merge `-work` into `-main` (after fast-forwarding `-main` from `main`); commit with `-s` and a conventional-commit message.
2. Non-fast-forward merge `-main` into `main` (produces a merge commit).
3. Branch `production-X.Y.Z` from `main`.

Force-push to `main` or `production-*` is not allowed. Force-push to
`-work` branches is acceptable. Branches stay in `origin` as archive
after merge.

### Remotes

Three remotes by convention; concrete URLs live in each operator's
local `.git/config`, not in this doc.

| Remote | Role |
|---|---|
| `origin` | Personal fork; primary read/write target during work. |
| `upstream` | Shared integration repo; receives merged work. |
| `github-origin` | Public mirror on GitHub. Prefix convention reserved for any external platform (`github-upstream`, …). |

### Commit Identity

Set `user.email` per-repo (not relying on the global default) so it
matches the GitHub Noreply (`<id>+<username>@users.noreply.github.com`).
Register the same address as a **secondary** e-mail on the shared/
internal forge profile if commits should link to the account there
too.

### Commit Messages

Conventional commits: `feat:`, `fix:`, `refactor:`, `docs:`,
`chore:`, `test:`. Sign off with `-s`.

## Push Policy for `github-*` Remotes

`github-*` remotes are the strictly controlled tier. Two gates apply:
a **branch whitelist** (which refs may be pushed at all) and an
**anonymization audit** (what their content may contain).

### Branch Whitelist

| Branch class | Allowed on `github-*`? |
|---|---|
| `main` | always |
| `production-X.Y.Z` | always |
| `feature-<name>-{main,work}` | only after explicit operator consent |
| `development-X.Y.Z-{main,work}` | **never** |
| `fix-*`, `hotfix-*` | not on `github-*` unless explicitly asked; treat like `feature-*` |

Release flow: merge `development-X.Y.Z-work` → `-main` → `main`,
branch `production-X.Y.Z` from `main`, *then* push `main` +
`production-X.Y.Z` to `github-*`. Development history never lands
there.

If a forbidden branch ever shows up on `github-*` (mis-push, stale
tracking ref), delete it remotely
(`git push github-<remote> --delete <branch>`) and prune locally
(`git fetch <remote> --prune`).

### Anonymization Audit

**Never** push commits — neither HEAD nor in the reachable history
— to a `github-*` remote that contain:

- Personal e-mails other than the configured GitHub Noreply.
- Real IPs from operator infrastructure (internal ranges or per-VM
  public IPs).
- Internal hostnames or personal/organizational domain names.
- BattleGroup display names or Sietch names tied to live deployments.
- Any other operator-specific identifier.

**Before any push to a `github-*` remote**, audit all commits that
would be uploaded:

```sh
git log <previous-pushed>..HEAD -p | grep -nE '<patterns>'
```

If hits are found, anonymize and *squash* the fix into its
introducing commit (`git rebase -i` + `fixup`) so the unanonymized
strings never reach the public history. Placeholder values:

- IPs: `192.0.2.x` (RFC 5737 TEST-NET-1, reserved for examples).
- Hostnames: `vm-host-NN`.

Pushes to `origin` and `upstream` carry no such restriction. They
are the working / integration remotes and accept everything; only
the GitHub-side tier gets the audit gate.
