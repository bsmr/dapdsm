package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

// Run starts the background poller, the hub→program bridge, and the
// foreground bubbletea program (alt-screen). It blocks until the user quits
// or ctx is cancelled. Wiring only — the model/bridge carry the logic.
func Run(ctx context.Context, c *core.Core) error {
	// Child context cancelled when Run returns (e.g. the user quits the
	// program), so the poller and the frame-forwarder goroutine below stop
	// even when the parent ctx (the signal context) is still live.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.Poller.Run(ctx)

	p := tea.NewProgram(
		newModel(ctx, c),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	hosts := listHostNames(c)
	frames, unsub := subscribeHosts(c.Hub, hosts)
	defer unsub()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-frames:
				if !ok {
					return
				}
				p.Send(msg)
			}
		}
	}()

	_, err := p.Run()
	return err
}
