package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
)

var errAbort = errors.New("bring-up aborted; run 'ds-arrakis doctor' to check prerequisites")

type resolveInput struct {
	Flags        config.Override
	FLSTokenFlag string
	BGNameFlag   string
	Found        *config.Config
	FoundExists  bool
	NoInput      bool
}

type resolved struct {
	Cfg          config.Config
	BGName       string
	FLSTokenPath string
}

func (in resolveInput) flagsComplete() bool {
	return in.Flags.WorldName != "" && in.Flags.WorldRegion != "" &&
		in.FLSTokenFlag != "" && in.BGNameFlag != ""
}

// resolveConfig implements the §6 state machine.
func resolveConfig(in resolveInput, stdin *bufio.Reader, stdout io.Writer) (resolved, error) {
	switch {
	case in.flagsComplete() && !in.FoundExists:
		// Case 1: scriptable, no conflict possible.
		return assemble(config.Config{}, in), nil

	case in.FoundExists:
		conflicts := diffKeys(in.Flags, *in.Found)
		if len(conflicts) == 0 {
			// Case 4: use cluster (+ any complementary flags).
			return assemble(*in.Found, in), nil
		}
		// Case 3: conflict — show diff, offer wizard.
		fmt.Fprintln(stdout, "bringup: flags differ from the cluster config:")
		for _, c := range conflicts {
			fmt.Fprintf(stdout, "  %s\n", c)
		}
		if !offerWizard(in, stdin, stdout) {
			return resolved{}, errAbort
		}
		return assemble(config.Merge(*in.Found, in.Flags), in), nil

	default:
		// Case 2: nothing — offer wizard.
		if !offerWizard(in, stdin, stdout) {
			return resolved{}, errAbort
		}
		return assemble(config.Config{}, in), nil
	}
}

// assemble layers the bring-up flags over a base config (flags win) and pulls
// BG name / token path from flags.
func assemble(base config.Config, in resolveInput) resolved {
	cfg := config.Merge(base, in.Flags)
	if cfg.Target == "" {
		cfg.Target = config.TargetProd
	}
	return resolved{Cfg: cfg, BGName: in.BGNameFlag, FLSTokenPath: in.FLSTokenFlag}
}

// offerWizard returns true if the operator accepts and completes the wizard.
// A non-interactive invocation (NoInput) never prompts: it declines.
func offerWizard(in resolveInput, stdin *bufio.Reader, stdout io.Writer) bool {
	if in.NoInput {
		return false
	}
	fmt.Fprint(stdout, "Run the bring-up wizard? [y/N]: ")
	ans, _ := stdin.ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(ans), "y")
}

// diffKeys lists the bring-up fields where a non-empty flag disagrees with the
// found config.
func diffKeys(flags config.Override, found config.Config) []string {
	var out []string
	if flags.WorldName != "" && flags.WorldName != found.WorldName {
		out = append(out, fmt.Sprintf("WorldName: flag=%q cluster=%q", flags.WorldName, found.WorldName))
	}
	if flags.WorldRegion != "" && flags.WorldRegion != found.WorldRegion {
		out = append(out, fmt.Sprintf("WorldRegion: flag=%q cluster=%q", flags.WorldRegion, found.WorldRegion))
	}
	if flags.ServerDisplayName != "" && flags.ServerDisplayName != found.ServerDisplayName {
		out = append(out, fmt.Sprintf("ServerDisplayName: flag=%q cluster=%q", flags.ServerDisplayName, found.ServerDisplayName))
	}
	if flags.HostDatacenterID != "" && flags.HostDatacenterID != found.HostDatacenterID {
		out = append(out, fmt.Sprintf("HostDatacenterID: flag=%q cluster=%q", flags.HostDatacenterID, found.HostDatacenterID))
	}
	return out
}

// promptValue prompts the operator for a single value, showing the default in
// brackets when one exists. Empty input returns def unchanged.
func promptValue(stdin *bufio.Reader, stdout io.Writer, label, def string) string {
	if def != "" {
		fmt.Fprintf(stdout, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(stdout, "%s: ", label)
	}
	line, _ := stdin.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}
