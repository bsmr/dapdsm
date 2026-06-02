// Package tui implements dunemgr's full-screen terminal UI — a second
// operator frontend beside the web UI, built on the shared core + command
// dispatcher (SP1). It runs the status poller in the background and renders
// live BattleGroup status in panes, with a ':' command bar that feeds the
// same command.Dispatch the CLI uses. An operator reaches it over SSH (the
// web UI's localhost bind is unreachable from a tablet).
//
// v1 limitation: the live-status subscription is resolved at startup, so a
// host added via the command bar during a session streams live only after a
// restart (its commands still work, and the poller still probes it).
package tui

import (
	"bytes"
	"context"
	"strings"

	spinner "github.com/charmbracelet/bubbles/spinner"
	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
)

// mode is the input mode of the TUI.
type mode int

const (
	modeNav mode = iota // navigate panes / select host
	modeCmd             // typing into the ':' command bar
)

// hostStatus is the latest known status of one host, folded from poll frames.
type hostStatus struct {
	bgState      string // e.g. RUNNING, DEGRADED, UNKNOWN
	ready, total int
	reachable    bool
	err          string
	lastAction   string // most recent action/result line
}

// pollKind tags a poll frame's channel.
type pollKind int

const (
	pollBG pollKind = iota
	pollHealth
	pollAction
)

// pollMsg is one status frame forwarded from the SSE hub by the bridge.
type pollMsg struct {
	host      string
	kind      pollKind
	bgState   string
	ready     int
	total     int
	reachable bool
	err       string
	action    string
	result    string
}

// cmdResultMsg carries the captured output of a finished command-bar dispatch.
type cmdResultMsg struct {
	out string
	err error
}

// playerNamesMsg delivers a freshly fetched slice of character names for one host.
type playerNamesMsg struct {
	host  string
	names []string
}

// refreshVerb is the TUI built-in that drops the player-name cache for the
// selected host (handled in the model, not the dispatcher).
const refreshVerb = "refresh"

// focusPane identifies which top-level pane has focus for cosmetic highlighting.
type focusPane int

const (
	focusHosts  focusPane = iota // host list pane
	focusEvents                  // event log pane
)

// model is the bubbletea root model.
type model struct {
	ctx  context.Context
	core *core.Core

	width, height int
	mode          mode
	focus         focusPane // which top pane is visually focused

	hosts    []string
	statuses map[string]hostStatus
	selected int
	events   []string

	input     textinput.Model
	output    string // last command result pane
	outScroll int    // scroll offset into the output pane (lines)
	history   []string
	histIdx   int

	running     bool                // true while a dispatched command is in-flight
	spin        spinner.Model       // animated spinner shown while running
	playerNames map[string][]string // per-host player-name cache for argPlayer completion
}

const maxEvents = 200

// outputWindow is the number of lines shown at once in the output pane.
const outputWindow = 10

// newModel builds the root model. ctx/core may be nil in unit tests that only
// exercise key handling that does not dispatch.
func newModel(ctx context.Context, c *core.Core) model {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.CharLimit = 512
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	m := model{
		ctx:         ctx,
		core:        c,
		mode:        modeNav,
		hosts:       listHostNames(c),
		statuses:    map[string]hostStatus{},
		input:       ti,
		spin:        sp,
		playerNames: map[string][]string{},
	}
	return m
}

// parseLine splits a command-bar line into argv on whitespace.
func parseLine(line string) []string { return strings.Fields(line) }

// selectedHost returns the currently-selected host name, or "" if none.
func (m model) selectedHost() string {
	if len(m.hosts) > 0 && m.selected >= 0 && m.selected < len(m.hosts) {
		return m.hosts[m.selected]
	}
	return ""
}

// dispatch runs the command line against the shared core on a goroutine and
// reports the captured output as a cmdResultMsg. It must be returned as a
// tea.Cmd so Update never blocks on SSH.
func (m model) dispatch(line string) tea.Cmd {
	c := m.core
	ctx := m.ctx
	line = injectHost(line, m.selectedHost(), m.hosts)
	return func() tea.Msg {
		argv := parseLine(line)
		if len(argv) == 0 {
			return cmdResultMsg{}
		}
		var buf bytes.Buffer
		err := command.Dispatch(ctx, c, argv, &buf, &buf)
		return cmdResultMsg{out: buf.String(), err: err}
	}
}

// fetchPlayerNames fetches character names for host via PlayerSearch and returns
// the result as a playerNamesMsg. On error an empty msg is returned so the cache
// entry simply stays absent (the operator can retry with :refresh).
func (m model) fetchPlayerNames(host string) tea.Cmd {
	c := m.core
	ctx := m.ctx
	return func() tea.Msg {
		if c == nil {
			return playerNamesMsg{host: host}
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		players, err := r.PlayerSearch(ctx, host, "%", 200)
		if err != nil {
			return playerNamesMsg{host: host}
		}
		names := make([]string, 0, len(players))
		for _, p := range players {
			if p.CharacterName != "" {
				names = append(names, p.CharacterName)
			}
		}
		return playerNamesMsg{host: host, names: names}
	}
}

// currentSlotIsPlayer reports whether the in-progress token of line occupies
// an argPlayer slot of its verb, accounting for an implied host. hosts is the
// full configured host list; a non-empty slice activates the implied-host
// shift (effectiveArgPos). The caller must separately ensure a host is
// actually selected before acting on the result.
func currentSlotIsPlayer(line string, hosts []string) bool {
	tokens, _ := splitCurrent(line)
	if len(tokens) == 0 {
		return false
	}
	spec, ok := command.SpecFor(tokens[0])
	if !ok {
		return false
	}
	return spec.IsPlayerPos(effectiveArgPos(spec, tokens, hosts))
}

// listHostNames returns the list of host names from the store, or nil if c is nil.
func listHostNames(c *core.Core) []string {
	if c == nil {
		return nil
	}
	profiles, err := c.Store.ListHosts()
	if err != nil {
		return nil
	}
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	return names
}

func (m model) Init() tea.Cmd { return nil }

// Update handles one message. It must never block: any I/O is deferred to a
// tea.Cmd.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		// Command mode is handled first so nav keys (q, arrows) don't leak
		// while the user is typing, and ':' in the bar doesn't re-trigger
		// mode switching.
		if m.mode == modeCmd {
			switch msg.Type {
			case tea.KeyEsc:
				m.mode = modeNav
				m.input.Blur()
				return m, nil
			case tea.KeyTab:
				completed, _ := complete(m.input.Value(), m.hosts, m.selectedHost(), m.playerNames)
				m.input.SetValue(completed)
				m.input.CursorEnd()
				if h := m.selectedHost(); h != "" && currentSlotIsPlayer(m.input.Value(), m.hosts) {
					if _, ok := m.playerNames[h]; !ok {
						return m, m.fetchPlayerNames(h)
					}
				}
				return m, nil
			case tea.KeyUp:
				if m.histIdx > 0 {
					m.histIdx--
					m.input.SetValue(m.history[m.histIdx])
					m.input.CursorEnd()
				}
				return m, nil
			case tea.KeyDown:
				if m.histIdx < len(m.history)-1 {
					m.histIdx++
					m.input.SetValue(m.history[m.histIdx])
					m.input.CursorEnd()
				} else {
					m.histIdx = len(m.history)
					m.input.SetValue("")
				}
				return m, nil
			case tea.KeyEnter:
				line := m.input.Value()
				if strings.TrimSpace(line) != "" {
					m.history = append(m.history, line)
				}
				m.histIdx = len(m.history)
				m.input.SetValue("")
				m.input.Blur()
				m.mode = modeNav
				if fields := strings.Fields(line); len(fields) > 0 {
					switch fields[0] {
					case helpVerb:
						m.output = renderHelp(fields[1:])
						return m, nil
					case refreshVerb:
						h := m.selectedHost()
						delete(m.playerNames, h)
						if h != "" {
							m.output = "completion cache refreshed for " + h
						} else {
							m.output = "completion cache refreshed"
						}
						return m, nil
					}
				}
				m.running = true
				return m, tea.Batch(m.dispatch(line), m.spin.Tick)
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		// modeNav key handling.
		// PgDown/PgUp scroll the output pane; checked by type before the
		// string switch so the bubbletea key-string representation doesn't
		// matter (the test exercises Type directly).
		switch msg.Type {
		case tea.KeyPgDown:
			lines := len(strings.Split(strings.TrimRight(m.output, "\n"), "\n"))
			maxScroll := lines - outputWindow
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.outScroll < maxScroll {
				m.outScroll++
			}
			return m, nil
		case tea.KeyPgUp:
			if m.outScroll > 0 {
				m.outScroll--
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down", "j":
			if m.selected < len(m.hosts)-1 {
				m.selected++
			}
			return m, nil
		case ":":
			m.mode = modeCmd
			m.input.SetValue("")
			if h := m.selectedHost(); h != "" {
				m.input.Prompt = "[" + h + "] › "
			} else {
				m.input.Prompt = "› "
			}
			return m, m.input.Focus()
		case "tab":
			if m.focus == focusHosts {
				m.focus = focusEvents
			} else {
				m.focus = focusHosts
			}
			return m, nil
		}
	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case cmdResultMsg:
		m.mode = modeNav
		m.outScroll = 0
		m.running = false
		if msg.err != nil {
			m.output = msg.out + "\nerror: " + msg.err.Error()
		} else {
			m.output = msg.out
		}
		return m, nil
	case playerNamesMsg:
		m.playerNames[msg.host] = msg.names
		return m, nil
	case pollMsg:
		s := m.statuses[msg.host]
		switch msg.kind {
		case pollBG:
			s.bgState, s.ready, s.total, s.err = msg.bgState, msg.ready, msg.total, msg.err
		case pollHealth:
			s.reachable, s.err = msg.reachable, msg.err
		case pollAction:
			s.lastAction = msg.action + " → " + msg.result
			m.events = appendCapped(m.events, msg.host+": "+s.lastAction, maxEvents)
		}
		m.statuses[msg.host] = s
		return m, nil
	}
	return m, nil
}

// appendCapped appends s to log, keeping at most max entries (drops oldest).
func appendCapped(log []string, s string, max int) []string {
	log = append(log, s)
	if len(log) > max {
		log = log[len(log)-max:]
	}
	return log
}

func (m model) View() string {
	list := renderHostList(m.hosts, m.statuses, m.selected)
	eventLog := renderEvents(m.events, 20)
	var detail string
	if len(m.hosts) > 0 && m.selected < len(m.hosts) {
		h := m.hosts[m.selected]
		detail = renderDetail(h, m.statuses[h])
	}
	var outputPane string
	if m.output != "" {
		outputPane = renderOutputScrolled(m.output, m.outScroll, outputWindow)
	}
	var bottom string
	if m.mode == modeCmd {
		bottom = m.input.View() + "\n"
		if sugg := suggest(m.input.Value(), m.hosts, m.selectedHost(), m.playerNames); len(sugg) > 0 {
			bottom += styleErr.Render(renderSuggestions(sugg)) + "\n"
		}
		if hint := usageHint(m.input.Value()); hint != "" {
			bottom += styleErr.Render(hint) + "\n"
		}
	} else {
		bottom = "[:] command  [tab] focus  [q] quit\n"
	}
	if m.running {
		bottom += m.spin.View() + " running…\n"
	}

	if m.width == 0 {
		// Fallback for tests and the very first frame before WindowSizeMsg.
		return list + "\n" + eventLog + "\n" + detail + "\n" + outputPane + bottom
	}

	// Bordered sidebar (hosts + recent events) left, content right, command block bottom.
	sidebar := styleBox.Width(sidebarWidth).Render(list + "\n" + renderEvents(m.events, 6))
	contentW := m.width - sidebarWidth - 6 // two borders + gap
	if contentW < 20 {
		contentW = 20
	}
	content := styleBox.Width(contentW).Render(detail + "\n" + outputPane)
	top := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	cmdW := m.width - 2
	if cmdW < 20 {
		cmdW = 20
	}
	cmdBlock := styleBox.Width(cmdW).Render(strings.TrimRight(bottom, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, top, cmdBlock)
}
