package cli

import (
	"context"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/lifecycle"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// lifecycleCmd drives a BattleGroup lifecycle verb (start|stop|restart|update)
// on a named host by dispatching the action through the lifecycle.Runner.
func lifecycleCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr lifecycle <host> <start|stop|restart|update>")
		return fmt.Errorf("lifecycle: usage: %w", ErrUsage)
	}
	host := args[0]
	action, err := lifecycle.ValidateAction(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	r := &lifecycle.Runner{
		SSH:   ssh.NewClient(),
		Store: st,
	}
	res, err := r.Run(ctx, "cli", host, action)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "lifecycle %s on %s ok\n%s", res.Action, res.Host, res.Stdout)
	return nil
}
