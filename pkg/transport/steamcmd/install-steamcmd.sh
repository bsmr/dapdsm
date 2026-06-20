#!/usr/bin/env bash
# Install steamcmd from the host distribution's package manager and make
# it reachable for the dune user under a non-interactive PATH.
#
# Why both steps:
# - Debian-family steamcmd packages install the binary under /usr/games/,
#   which is on an interactive shell's PATH but not on a non-interactive
#   PATH. Funcom's `battlegroup update` invokes `steamcmd` by bare name
#   and runs non-interactively, so it needs the binary on a path that is
#   guaranteed to be present (we use /usr/local/bin via symlink).
# - On Debian (vs. Ubuntu) the package lives in the non-free component
#   and pulls 32-bit libraries, so the i386 architecture must be enabled
#   and non-free must be added to apt sources first.
#
# Idempotent: safe to re-run.

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  exec sudo -- "$0" "$@"
fi

if [ ! -r /etc/os-release ]; then
  echo "install-steamcmd: /etc/os-release missing — cannot detect distro" >&2
  exit 1
fi
# shellcheck disable=SC1091
. /etc/os-release

# The steamcmd package prompts for the Steam Subscriber Agreement via
# debconf. Preseed an "I AGREE" answer so the install runs non-interactive.
preseed_steam_license() {
  debconf-set-selections <<<"steam steam/question select I AGREE"
  debconf-set-selections <<<"steam steam/license note ''"
}

case "${ID:-}" in
  ubuntu)
    dpkg --add-architecture i386
    add-apt-repository -y multiverse
    apt-get update
    preseed_steam_license
    DEBIAN_FRONTEND=noninteractive apt-get install -y steamcmd
    ;;
  debian)
    dpkg --add-architecture i386
    sources_drop_in=/etc/apt/sources.list.d/steamcmd-non-free.sources
    if [ ! -e "${sources_drop_in}" ]; then
      cat > "${sources_drop_in}" <<EOF
Types: deb
URIs: http://deb.debian.org/debian/
Suites: ${VERSION_CODENAME} ${VERSION_CODENAME}-updates
Components: non-free contrib
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg
EOF
    fi
    apt-get update
    preseed_steam_license
    DEBIAN_FRONTEND=noninteractive apt-get install -y steamcmd
    ;;
  *)
    echo "install-steamcmd: unsupported distro '${ID:-unknown}'" >&2
    echo "                  Add a branch for it here once verified." >&2
    exit 1
    ;;
esac

# /usr/games is not on the non-interactive PATH on either distro.
# Symlink to /usr/local/bin so Funcom's `battlegroup update` (which
# calls `steamcmd` by bare name) can find it.
ln -sfn /usr/games/steamcmd /usr/local/bin/steamcmd

echo
echo "install-steamcmd: $(/usr/local/bin/steamcmd +quit 2>&1 | head -1 || true)"
