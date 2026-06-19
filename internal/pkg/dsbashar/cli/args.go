package cli

import (
	"flag"
	"strings"
)

// boolFlag is re-declared from the stdlib flag package (where it is
// unexported) so we can detect bool flags via type assertion below.
type boolFlag interface {
	flag.Value
	IsBoolFlag() bool
}

// reorderFlagArgs returns args with all recognised flags (and their
// value-args when applicable) moved before the positional arguments,
// so Go's flag.Parse — which stops at the first non-flag — accepts
// intermixed CLI calls like `ds-bashar ini-set KEY VALUE --apply`.
//
// Strategy: scan args once, classify each as flag-or-value by looking
// it up in fs. A flag carries a separate value-arg when (a) it is not
// bool and (b) it does not use the --name=value form.
//
// The POSIX `--` end-of-flags marker terminates classification and the
// remainder is treated as positional.
func reorderFlagArgs(fs *flag.FlagSet, args []string) []string {
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positionals = append(positionals, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		if bf, ok := f.Value.(boolFlag); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positionals...)
}
