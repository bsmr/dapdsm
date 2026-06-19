#!/usr/bin/env bash
# Deploy ds-bashar (and dapdsm-side config templates) to a target VM.
#
# Standalone: runs from a fresh `dapdsm` clone, without the meta-repo.
#   - cross-build ds-bashar for linux/amd64 with VCS metadata baked in
#   - rsync this repo's etc/ to /opt/dapdsm on the host
#   - install the binary at /usr/local/bin/ds-bashar
#
# Idempotent. Run from the operator workstation, with SSH access to the
# target VM as the dune user (see "Access Model" in CLAUDE.md).
#
#   scripts/deploy.sh <ssh-host-or-alias>
#
# The host must already have:
#   - the dune user with passwordless sudo
#   - /etc/dune/dunectl.env (copy from etc/dune/dunectl.env.example)
#
# This script does not touch /etc/dune/, /etc/rancher/k3s/, or the
# BattleGroup state — those are operator decisions, applied separately.
# Host bootstrap (K3s installer, SteamCMD, Funcom operator images) is
# orthogonal to ds-bashar and lives in the meta-repo orchestrator.

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <ssh-host-or-alias>" >&2
  exit 1
fi

readonly HOST="$1"
readonly REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly BIN="${REPO_ROOT}/bin/ds-bashar-linux-amd64"

cd "${REPO_ROOT}"

echo "[1/4] go vet"
go vet ./...

echo "[2/4] cross-build ds-bashar for linux/amd64"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "${BIN}" ./cmd/ds-bashar

echo "[3/4] sync etc/ to dune@${HOST}:/opt/dapdsm"
tar -c etc | ssh -o BatchMode=yes "dune@${HOST}" \
  'sudo install -d -m 0755 /opt/dapdsm && sudo tar -x -C /opt/dapdsm && sudo chown -R root:root /opt/dapdsm'

echo "[4/4] install /usr/local/bin/ds-bashar"
scp -o BatchMode=yes "${BIN}" "dune@${HOST}:/tmp/ds-bashar"
ssh -o BatchMode=yes "dune@${HOST}" \
  'sudo install -m 0755 /tmp/ds-bashar /usr/local/bin/ds-bashar && rm -f /tmp/ds-bashar'

echo
ssh -o BatchMode=yes "dune@${HOST}" 'ds-bashar version'
