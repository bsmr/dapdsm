# ds-bashar Multi-Node Bring-Up Operator Guide

The `ds-bashar bringup` verb orchestrates bringing a BattleGroup online on a multi-node cluster accessed via jumphost.
The local `dunectl.env` file is now a *bootstrap* input only; the cluster-resident ConfigMap and Secret become the source of truth after the first bring-up.

## Prerequisites

- A multi-node Kubernetes cluster reachable via SSH jumphost (configured in `~/.ssh/config`).
- Local `~/.ssh/config` sets up a ProxyJump chain to the jumphost and target VM (operator user `dune`).
- Jumphost has `/home/dune/kubeconfig` pointing to the cluster (or use the `--kubeconfig` flag).
- Funcom operator CRDs and images already installed on the cluster (pre-requisite from `ds-arrakis`).
- FLS Self-Host JWT token file on the workstation.

## Global Flags

Global flags `--jump` and `--kubeconfig` precede the verb name:

```sh
ds-bashar --jump <host-alias> --kubeconfig <path> <verb> [verb-args]
```

- `--jump <host-alias>` — SSH alias from `~/.ssh/config` that reaches the cluster's jumphost. Multi-node mode; without it, verbs use local on-node kubectl.
- `--kubeconfig <path>` — Override the default kubeconfig path (default: `$KUBECONFIG` env var, or `~/.kube/config`). When used with `--jump`, the path is on the jumphost.

Example:

```sh
ds-bashar --jump jh-prod --kubeconfig /home/dune/kubeconfig bringup \
  --name MyBG --display "My Server" --region Europe --fls-token /path/to/fls.jwt
```

## Bring-Up Flow

The `bringup` subcommand executes the full multi-node bring-up pipeline:

1. **Resolve configuration** — merge flags, local `dunectl.env` (bootstrap input), and any existing cluster config.
2. **Interactive wizard** (if needed) — when flags are incomplete and `--no-input` is not set, prompts for missing values.
3. **Promote to cluster** — write the config to the cluster-resident ConfigMap `dapdsm-bg-config` and Secret `dapdsm-bg-secrets` in namespace `dapdsm-system`.
4. **Discovery gate** — check for existing BattleGroups.
5. **Run Funcom setup.sh** — only on fresh clusters (no BattleGroups yet); skipped if one already exists.
6. **Load BG runtime image** — HTTP-fetch the 4.2 GB image from the jumphost file server to the in-cluster image importer DaemonSet.
7. **Initialize database** — run `init-db` to create the per-app Postgres role and database.
8. **Reconcile** — apply the declarative post-bootstrap pipeline (enable sets, patch ports, ini-set, etc.).

### Minimal Bring-Up Invocation

Provide the three required flags to skip the wizard:

```sh
ds-bashar --jump jh-prod bringup \
  --name MyBG \
  --display "My Server" \
  --region Europe \
  --fls-token /home/user/fls-token-prod
```

Supported regions: `Asia`, `Europe`, `North America`, `Oceania`, `South America`.

### Interactive Wizard

When any required flag is missing and `--no-input` is not set, `bringup` offers an interactive wizard:

```sh
ds-bashar --jump jh-prod bringup
```

The wizard prompts for:
- BattleGroup name (WorldName) — required, YAML-safe (alphanumeric, underscore, hyphen only).
- World region — required, one of the Funcom-recognized regions.
- Server display name — optional; can be set later via `ini-set Bgd.ServerDisplayName`.
- FLS token path — required; must be readable on the workstation.

Press `Ctrl-C` or answer `n` to cancel and abort the wizard. Aborting returns an error with a hint: run `ds-arrakis doctor` to check prerequisites.

### No-Input Mode

Use `--no-input` to fail instead of prompting. Useful in CI/automation:

```sh
ds-bashar --jump jh-prod bringup \
  --name MyBG \
  --display "My Server" \
  --region Europe \
  --fls-token /path/to/fls.jwt \
  --no-input
```

If required flags are missing, the command exits with an error (no wizard).

### Config Resolution State Machine

The wizard handles three cases:

1. **All flags provided, no cluster config** — Use the flags directly (scriptable).
2. **No flags, no cluster config** — Offer the wizard.
3. **Flags and cluster config exist** — Compare them.
   - If they match, use the cluster config.
   - If they conflict, show the differences and offer the wizard.
   - User can accept (merge with updated flags taking priority) or cancel.

### Cluster-Resident Configuration

After the first successful `bringup`, the cluster ConfigMap `dapdsm-bg-config` and Secret `dapdsm-bg-secrets` (namespace `dapdsm-system`) become the source of truth.
Subsequent `bringup` runs pull this existing config and compare it against any new flags.

The local `/etc/dune/dunectl.env` on the jumphost is no longer consulted during `bringup` — it serves only as a bootstrap reference.
To update cluster config after bring-up, either:
- Provide new flags and let the wizard merge them, or
- Edit the ConfigMap/Secret directly with `kubectl`.

### Non-Idempotent Setup Gate

Funcom's `setup.sh` is a one-time initialization (creates the initial BattleGroup CR, database schema, etc.) and cannot be safely re-run on an existing cluster.

The `bringup` orchestration checks for existing BattleGroups:
- **Found:** skips `setup.sh`.
- **None found:** runs `setup.sh` to initialize the cluster.

If you need to reset the cluster or re-run `setup.sh`, manually delete the BattleGroup CR(s) first.

## BattleGroup Runtime Image Loading

The BattleGroup runtime image (4.2 GB) is loaded via HTTP-fetch from a jumphost file server to the in-cluster importer DaemonSet.
This requires pod→jumphost network reachability, which is confirmed at the live test.

**Caveat:** The image must be staged on the jumphost at a location the importer pods can reach via HTTP.
If this fails, check network policies and firewall rules between the cluster and jumphost.

A fallback mechanism to pull from an in-cluster registry is planned but not yet implemented.
For now, HTTP-fetch from the jumphost is the primary path.

## The `discover` Subcommand

List all BattleGroups found on the cluster:

```sh
ds-bashar --jump jh-prod discover
```

Output:
```
discover: 2 BattleGroup(s):
  - arrakis
  - caladan
```

Or, if the cluster is empty:
```
discover: no BattleGroups found (cluster has none yet)
```

This is useful to verify what's already running before planning a new bring-up.

## After Bring-Up

Once `bringup` completes successfully:

- The BattleGroup is created and starting up.
- Player connectivity is established through the public IP (configured on the node as ExternalIP).
- Monitor BattleGroup status with `ds-bashar --jump jh-prod list-sets` (show maps and scaling mode).
- Further configuration via `ini-set`, `enable-set`, `disable-set`, `start`, `stop`, etc. — all follow the same `--jump` pattern.

For the full list of available subcommands, run:

```sh
ds-bashar help
```

## Configuration File Reference

The local `/etc/dune/dunectl.env` (or a copy on the workstation) is a *bootstrap* input for:
- Initial values when `bringup` is invoked (used to seed the config if no cluster config exists yet).
- Local operator commands (e.g., single-node `reconcile`, `ini-set`) on the host VM.

**After the first `bringup`**, this file is no longer the source of truth.
The cluster ConfigMap `dapdsm-bg-config` and Secret `dapdsm-bg-secrets` are authoritative.

Edits to `dunectl.env` after bring-up will not propagate to the cluster; changes must be made via:
- Flags + wizard in a new `bringup` invocation (which updates the cluster), or
- Direct kubectl edits to the ConfigMap/Secret.

See `etc/dune/dunectl.env.example` in the repository for the full list of bootstrap keys.
