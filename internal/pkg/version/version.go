// Package version exposes the dapdsm build identity (semantic version, VCS
// revision, build date, Go runtime) shared by both binaries, dunectl and
// dunemgr — they live in one module and release in lockstep, so they carry one
// version. Values come from one of three sources, in order of precedence:
//
//  1. ldflags injected by the build (-X .../version.Commit=abc, etc.)
//  2. Go's automatic vcs.* build settings (debug.ReadBuildInfo) — populated
//     whenever the binary is built from a clean git tree, no flags needed.
//  3. Compile-time defaults — "dev" for Version, empty for Commit/Date.
package version

import (
	"runtime"
	"runtime/debug"
)

// Semantic version of the dapdsm tools (dunectl + dunemgr). Bump per release.
var Version = "0.1.8"

// Short git revision. Empty when built from an unclean tree or outside git.
var Commit = ""

// Build/commit timestamp in RFC 3339. Empty when unknown.
var Date = ""

// Modified is "true" when the binary was built from a dirty working tree.
var Modified = ""

func init() {
	if Commit != "" {
		// ldflags already populated everything; do not override.
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 12 {
				Commit = s.Value[:12]
			} else {
				Commit = s.Value
			}
		case "vcs.time":
			Date = s.Value
		case "vcs.modified":
			Modified = s.Value
		}
	}
}

// String renders one human-readable line summarising the build identity for
// the named tool (e.g. "dunectl" or "dunemgr").
func String(tool string) string {
	s := tool + " " + Version
	if Commit != "" {
		s += " (" + Commit
		if Modified == "true" {
			s += "-dirty"
		}
		s += ")"
	}
	if Date != "" {
		s += " built " + Date
	}
	s += " " + runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH
	return s
}
