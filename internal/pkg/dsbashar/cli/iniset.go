package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"go.muehmer.eu/dapdsm/pkg/transport/iniconf"
)

// DefaultEngineINI is the Funcom-shipped INI file for battlegroup-wide
// console variables (server name, password, gameplay multipliers).
const DefaultEngineINI = "/home/dune/.dune/download/scripts/setup/config/UserEngine.ini"

// DefaultSection is the section that holds Bgd.* console variables and
// gameplay knobs. The UserGame.ini sections (UE-class paths) must be
// passed explicitly via --section.
const DefaultSection = "ConsoleVariables"

type iniSetDeps struct {
	readFile  func(path string) ([]byte, error)
	writeFile func(path string, content []byte, mode os.FileMode) error
	runVendor vendorRunner
}

func defaultIniSetDeps() iniSetDeps {
	return iniSetDeps{
		readFile: os.ReadFile,
		writeFile: func(path string, content []byte, mode os.FileMode) error {
			return os.WriteFile(path, content, mode)
		},
		runVendor: execVendor,
	}
}

func iniSetCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return iniSet(ctx, args, stdout, stderr, defaultIniSetDeps())
}

func iniSet(ctx context.Context, args []string, stdout, stderr io.Writer, deps iniSetDeps) error {
	fs := flag.NewFlagSet("ini-set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", DefaultEngineINI, "Path to the INI file to edit")
	section := fs.String("section", DefaultSection, "INI section name (without brackets)")
	raw := fs.Bool("raw", false, "Pass value through verbatim — do not auto-quote strings")
	apply := fs.Bool("apply", false, "After the edit, run 'battlegroup apply-default-usersettings'")
	restart := fs.Bool("restart", false, "After --apply, also run 'battlegroup restart' (implies --apply)")
	bgBin := fs.String("bg-binary", DefaultBattlegroupBin, "Path to Funcom's battlegroup wrapper (for --apply/--restart)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: ds-bashar ini-set [flags] <key> <value>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(reorderFlagArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("ini-set: %w: %w", ErrUsage, err)
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "ini-set: need exactly two positional arguments: <key> <value>")
		fs.Usage()
		return ErrUsage
	}
	key, value := fs.Arg(0), fs.Arg(1)

	written := value
	if !*raw && iniconf.NeedsQuoting(value) {
		written = iniconf.Quote(value)
	}

	content, err := deps.readFile(*file)
	if err != nil {
		return fmt.Errorf("read %s: %w", *file, err)
	}
	updated, err := iniconf.SetKey(content, *section, key, written)
	if err != nil {
		return fmt.Errorf("update %s: %w", *file, err)
	}
	if err := deps.writeFile(*file, updated, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *file, err)
	}
	fmt.Fprintf(stdout, "%s: [%s] %s=%s\n", *file, *section, key, written)

	if *apply || *restart {
		if err := deps.runVendor(ctx, *bgBin, "apply-default-usersettings", stdout, stderr); err != nil {
			return err
		}
	}
	if *restart {
		if err := deps.runVendor(ctx, *bgBin, "restart", stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}
