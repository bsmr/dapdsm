# dapdsm

_Dune: Awakening_ private dedicated server management suite for
[Kubernetes](https://kubernetes.io/). Automates the operator
workflows around Funcom's pre-built game server operators running
on a private cluster.

> **Early-stage software.** Used in production on the ADESTIS RKE2 Lab
> server. Expect rough edges and breaking changes between minor versions.

## What this is

dapdsm is a set of Go CLI tools that drive a private _Dune: Awakening_
dedicated server cluster. It handles the Dune layer — BattleGroup setup,
image distribution, world creation, lifecycle, and config — on top of a
cluster you already have. It does not provision VMs, nodes, load balancers,
or the cluster itself.

## Prerequisites

### Cluster baseline (assumed, not provided by dapdsm)

| Component | Minimum spec per role |
|---|---|
| Control plane nodes (×3 for HA) | 4 vCPU, 8 GB RAM, 50 GB disk |
| Worker nodes (×3) | 8 vCPU, 32 GB RAM, 200 GB disk |
| Load balancer nodes (×2) | 2 vCPU, 2 GB RAM, 20 GB disk |

### Extra components

| Component | Purpose | Install |
|---|---|---|
| [HAProxy](https://www.haproxy.org/) + [Keepalived](https://www.keepalived.org/) | HA LB for the API server and game ports | distro packages |
| [local-path-provisioner](https://github.com/rancher/local-path-provisioner) | Default StorageClass (RKE2 does not bundle it) | `ds-arrakis storage local-path` |
| [cert-manager](https://cert-manager.io/) | TLS for internal services (optional) | helm |

### CLI tools (on the jumphost / control host)

| Tool | Purpose | Install |
|---|---|---|
| `kubectl` ≥ 1.29 | Cluster access | [docs](https://kubernetes.io/docs/tasks/tools/) |
| `wget` | Image fetch during bring-up | distro packages |
| Go ≥ 1.24 | Building / `go install` | [go.dev/dl](https://go.dev/dl/) |

### OS

Tested on: Ubuntu 24.04 (workers), Debian 13 Trixie (jumphost). Other
distributions supported through the distro-detection layer.

## Install

```bash
go install go.muehmer.eu/dapdsm/cmd/ds-arrakis@latest
go install go.muehmer.eu/dapdsm/cmd/ds-bashar@latest
go install go.muehmer.eu/dapdsm/cmd/ds-thumper@latest
go install go.muehmer.eu/dapdsm/cmd/dunemgr@latest
```

## Quick start

```bash
# 1. Distribute Funcom's depot images to all cluster nodes
ds-arrakis images load --jump dune@<jumphost> --kubeconfig ~/kubeconfig

# 2. Bring up a BattleGroup (prompts for display name, region, FLS token)
ds-bashar bringup --jump dune@<jumphost> --kubeconfig ~/kubeconfig

# 3. Start the reconcile loop (keeps the CR in sync)
ds-bashar reconcile --jump dune@<jumphost> --kubeconfig ~/kubeconfig
```

## Binaries

| Binary | Role |
|---|---|
| `ds-arrakis` | Cluster bring-up: depot acquisition, image distribution, CRD install, host checks |
| `ds-bashar` | BattleGroup management: bring-up, lifecycle (start/stop/restart/upgrade), reconcile |
| `ds-thumper` | Config wizard and workstation→VM secret rollout |
| `dunemgr` | Player-domain TUI: player management, grants, statistics |

## License

MIT — Copyright (c) 2026 Boris Mühmer.