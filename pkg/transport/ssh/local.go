package ssh

import "context"

// LocalExecer runs commands directly on the local machine, ignoring the host
// argument. It is the on-VM transport for tools that already run on the node
// (e.g. ds-bashar): kubectl/cat execute locally rather than over SSH.
//
// It satisfies the same method set as *Client (Run/RunWithStdin with a host
// parameter), so it is interchangeable with it wherever a host-addressed
// executor is expected.
type LocalExecer struct {
	// Runner is injectable for tests; nil falls back to NewRunner().
	Runner Runner
}

func (l LocalExecer) runner() Runner {
	if l.Runner == nil {
		return NewRunner()
	}
	return l.Runner
}

// Run executes name+args locally. host is accepted for interface compatibility
// and ignored.
func (l LocalExecer) Run(ctx context.Context, _ string, name string, args ...string) (Result, error) {
	return l.runner().Run(ctx, name, args...)
}

// RunWithStdin executes name+args locally with stdin piped. host is ignored.
func (l LocalExecer) RunWithStdin(ctx context.Context, _ string, stdin []byte, name string, args ...string) (Result, error) {
	return l.runner().RunWithStdin(ctx, stdin, name, args...)
}
