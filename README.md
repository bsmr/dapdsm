# dapdsm

_Dune: Awakening_ private dedicated server management suite for
[Kubernetes](https://kubernetes.io/). Automates the operator
workflows around Funcom's pre-built game server operators running
on a private cluster.

> **Early-stage software.** Used in production on the RKE2 Testing Station
> server. Expect rough edges and breaking changes between minor versions.
>
> **No warranty.** The architecture, node counts, sizing figures, and configuration
> shown in this document reflect one operator's live deployment on specific cloud
> hardware. They are provided for illustration only and carry no performance,
> stability, or compatibility guarantee. You are solely responsible for your
> infrastructure design, sizing, and configuration. The author accepts no liability
> for any damage, data loss, or costs arising from the use of this software or
> the information in this document.

## What this is

dapdsm is a set of Go CLI tools that drive a private _Dune: Awakening_
dedicated server cluster. It handles the Dune layer — BattleGroup setup,
image distribution, world creation, lifecycle, and config — on top of a
cluster you already have. It does not provision VMs, nodes, load balancers,
or the cluster itself.

## Reference topology

The diagram below reflects the author's live RKE2 setup. Your deployment
will differ; it is a sketch, not a prescription.

```
    Internet / game clients
    UDP 7777-7810   TCP 31982   TCP 11717 (director)
            |
            v
    +---------------------+
    |  Gateway / Firewall |   provider-managed; NAT + DNAT
    +----------+----------+
               |
               |  private cluster network
               v
    +----------+----------+
    |  Load balancers x3  |   HAProxy + nft  (Keepalived VIP)
    +----+------------+---+
         |            |
         v            v
    +----------+  +-------------------+
    | Control  |  |  Workers x3       |
    | plane x3 |->|  game pods        |
    | (RKE2)   |  |  (hostNetwork)    |
    +----------+  +-------------------+

  Management (SSH only — via Nebula mesh overlay network):

  Workstation --[Nebula]--> Jumphost --[ProxyJump]--> all nodes
                                  kubectl / dapdsm run here
```

> Game clients reach the cluster through the provider's gateway only.
> No management port is exposed to the internet. All operator access is
> SSH tunnelled through the Nebula mesh; no node is reachable from the
> public internet directly.

## Prerequisites

### Cluster baseline (assumed, not provided by dapdsm)

> **Speculative sizing.** The figures below are taken from one live deployment and have
> not been validated against any Funcom guideline or load benchmark. Actual requirements
> depend on player count, number of active maps, and your cloud provider's hardware
> characteristics. Treat them as a rough starting point, not a recommendation.

| Component | Spec used in the reference deployment |
|---|---|
| Control plane nodes (×3 for HA) | 4 vCPU, 8 GB RAM, 50 GB disk |
| Worker nodes (×3) | 8 vCPU, 32 GB RAM, 200 GB disk |
| Load balancer nodes (×3) | 2 vCPU, 2 GB RAM, 20 GB disk |

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

Tool names carry Dune-flavoured meaning:
_ARRAKIS_ (Automated Rancher Runtime And Kubernetes Init System — cluster bootstrap),
_Bashar_ (a military rank in the Dune universe — operations command),
_Thumper_ (the device that summons sandworms — deploys things).
`dunemgr` is a functional name.

### Tool workflow

The tools form a sequential pipeline from first setup to ongoing operations.

```
  +----------------------------------------------------------+
  |  1. Config bootstrap  (once per operator workstation)    |
  |                                                          |
  |  ds-thumper --> seal + ship config, FLS token, secrets   |
  +-----------------------------+----------------------------+
                                |
  +-----------------------------+----------------------------+
  |  2. Cluster bootstrap  (once per cluster)                |
  |                                                          |
  |  ds-arrakis doctor      --> verify host prerequisites    |
  |  ds-arrakis images load --> push Funcom images to nodes  |
  |  ds-arrakis storage     --> install StorageClass         |
  +-----------------------------+----------------------------+
                                |
  +-----------------------------+----------------------------+
  |  3. BattleGroup bring-up  (once per BattleGroup)        |
  |                                                          |
  |  ds-bashar bringup      --> create CRs + world setup     |
  +-----------------------------+----------------------------+
                                |
  +-----------------------------+----------------------------+
  |  4. Day-2 operations  (ongoing)                          |
  |                                                          |
  |  ds-bashar reconcile    --> keep BattleGroup in sync     |
  |  ds-bashar start / stop / restart / upgrade              |
  |  dunemgr                --> player management TUI        |
  +----------------------------------------------------------+
```

## License

MIT — Copyright (c) 2026 Boris Mühmer.