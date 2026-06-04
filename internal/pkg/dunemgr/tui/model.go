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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	spinner "github.com/charmbracelet/bubbles/spinner"
	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	admincatalog "go.muehmer.eu/dapdsm/internal/pkg/dunemgr/admin/catalog"
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

const (
	editKindQty     = "qty"
	editKindQuality = "quality"
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

// playersMsg delivers the player list fetched when descending into levelPlayers.
type playersMsg struct {
	players []dbquery.Player
	err     string
}

// invsMsg delivers the inventory breakdown fetched when descending into levelInventory.
type invsMsg struct {
	fls, char string
	invs      []dbquery.InvBreakdown
	err       string
}

// itemsMsg delivers the item list fetched when descending into levelItem.
type itemsMsg struct {
	typ   int
	items []dbquery.ItemRow
	err   string
}

// editDoneMsg is returned by the async item-mutation tea.Cmds (applyStack,
// applyQuality, applyDelete). A nil err means success.
type editDoneMsg struct{ err error }

// refreshVerb is the TUI built-in that drops the player-name cache for the
// selected host (handled in the model, not the dispatcher).
const refreshVerb = "refresh"

// model is the bubbletea root model.
type model struct {
	ctx  context.Context
	core *core.Core

	width, height int
	mode          mode

	hosts    []string
	statuses map[string]hostStatus
	events   []string // collected from poll frames; not currently surfaced (header carries live status)

	input          textinput.Model
	output         string // last command result pane
	outScroll      int    // scroll offset into the output pane (lines)
	outHeaderLines int    // sticky header lines at the top of output (not scrolled)
	history        []string
	histIdx        int

	running     bool                // true while a dispatched command is in-flight
	spin        spinner.Model       // animated spinner shown while running
	playerNames map[string][]string // per-host player-name cache for argPlayer completion

	// modal nav state
	nav     navState
	players []dbquery.Player
	invs    []dbquery.InvBreakdown
	items   []dbquery.ItemRow
	curFLS  string
	curChar string
	curType int
	navErr  string

	// item-level inline editing state
	editing       bool   // true while the inline qty/quality input is active
	editKind      string // "qty" or "quality"
	confirmDelete bool   // true while waiting for y/n on item delete

	loading  bool // true while a lazy nav-level load is in-flight
	showHelp bool // true while the ? help overlay is displayed

	sortCol  [4]int  // active sort column index (into levelColumns) per nav level
	sortDesc [4]bool // sort direction per nav level
}

const maxEvents = 200

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
	m.nav.counts[levelHosts] = len(m.hosts)
	m.sortCol[levelPlayers] = 0 // NAME
	m.sortCol[levelItem] = 1    // NAME
	return m
}

// parseLine splits a command-bar line into argv on whitespace.
func parseLine(line string) []string { return strings.Fields(line) }

// selectedHost returns the currently-selected host name, or "" if none.
// It derives directly from the nav cursor so the header and command bar
// always reflect the highlighted row without needing a separate commit step.
func (m model) selectedHost() string {
	if len(m.hosts) > 0 {
		i := m.nav.sel[levelHosts]
		if i >= 0 && i < len(m.hosts) {
			return m.hosts[i]
		}
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

// loadPlayers fetches the full player list for the currently selected host.
func (m model) loadPlayers() tea.Cmd {
	c, ctx, host := m.core, m.ctx, m.selectedHost()
	return func() tea.Msg {
		if c == nil || host == "" {
			return playersMsg{err: "load players failed"}
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		ps, err := r.PlayerSearch(ctx, host, "%", 200)
		if err != nil {
			return playersMsg{err: "load players failed"}
		}
		return playersMsg{players: ps}
	}
}

// loadInvs fetches the inventory breakdown for the currently selected player.
func (m model) loadInvs() tea.Cmd {
	c, ctx, host := m.core, m.ctx, m.selectedHost()
	var ref, char string
	if i := m.nav.sel[levelPlayers]; i >= 0 && i < len(m.players) {
		ref = m.players[i].FLSID
		char = m.players[i].CharacterName
	}
	return func() tea.Msg {
		if c == nil || ref == "" {
			return invsMsg{err: "player not found / load failed"}
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		bs, err := r.InventoryBreakdown(ctx, host, ref)
		if err != nil {
			return invsMsg{err: "load inventories failed"}
		}
		return invsMsg{fls: ref, char: char, invs: bs}
	}
}

// loadItems fetches the item list for the currently selected inventory type.
func (m model) loadItems() tea.Cmd {
	c, ctx, host, fls, typ := m.core, m.ctx, m.selectedHost(), m.curFLS, m.curType
	return func() tea.Msg {
		if c == nil || fls == "" {
			return itemsMsg{typ: typ, err: "load items failed"}
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		rows, err := r.InventoryItems(ctx, host, fls, typ)
		if err != nil {
			return itemsMsg{typ: typ, err: "load items failed"}
		}
		return itemsMsg{typ: typ, items: rows}
	}
}

// selectedItemID returns the ID of the currently highlighted item row, or 0.
func (m model) selectedItemID() int64 {
	if m.nav.level == levelItem {
		if i := m.nav.sel[levelItem]; i >= 0 && i < len(m.items) {
			return m.items[i].ID
		}
	}
	return 0
}

// applyStack dispatches an async stack-size mutation via the shared gated path.
func (m model) applyStack(itemID, stack int64) tea.Cmd {
	c, ctx, host := m.core, m.ctx, m.selectedHost()
	return func() tea.Msg {
		if c == nil {
			return editDoneMsg{}
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		return editDoneMsg{err: command.ApplyItemStack(ctx, r, c.Store, host, itemID, stack, false)}
	}
}

// applyQuality dispatches an async quality-level mutation via the shared gated path.
func (m model) applyQuality(itemID, q int64) tea.Cmd {
	c, ctx, host := m.core, m.ctx, m.selectedHost()
	return func() tea.Msg {
		if c == nil {
			return editDoneMsg{}
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		return editDoneMsg{err: command.ApplyItemQuality(ctx, r, c.Store, host, itemID, q, false)}
	}
}

// applyDelete dispatches an async item deletion via the shared gated path.
func (m model) applyDelete(itemID int64) tea.Cmd {
	c, ctx, host := m.core, m.ctx, m.selectedHost()
	return func() tea.Msg {
		if c == nil {
			return editDoneMsg{}
		}
		r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
		return editDoneMsg{err: command.ApplyItemDelete(ctx, r, c.Store, host, itemID, false)}
	}
}

// descend moves one level deeper and kicks off the lazy load for the new level.
// It clears the target level's stale data and sets loading=true so View never
// renders the previous level's rows during the SSH round-trip.
func (m model) descend() (tea.Model, tea.Cmd) {
	m.output = "" // leave any command-result view; show the nav list
	switch m.nav.level {
	case levelHosts:
		if !m.nav.descend() {
			return m, nil
		}
		m.players = nil
		m.nav.counts[levelPlayers] = 0
		m.loading = true
		return m, m.loadPlayers()
	case levelPlayers:
		if m.nav.sel[levelPlayers] >= len(m.players) {
			return m, nil
		}
		m.nav.descend()
		m.invs = nil
		m.nav.counts[levelInventory] = 0
		m.loading = true
		return m, m.loadInvs()
	case levelInventory:
		i := m.nav.sel[levelInventory]
		if i >= len(m.invs) {
			return m, nil
		}
		m.curType = m.invs[i].InventoryType
		m.nav.descend()
		m.items = nil
		m.nav.counts[levelItem] = 0
		m.loading = true
		return m, m.loadItems()
	}
	return m, nil
}

// breadcrumb builds the content-box title for nav mode.
func (m model) breadcrumb() string {
	parts := []string{m.selectedHost()}
	if m.nav.level >= levelInventory {
		parts = append(parts, m.curChar)
	}
	if m.nav.level >= levelItem {
		parts = append(parts, command.InventoryTypeName(m.curType))
	}
	var pos string
	if m.nav.counts[m.nav.level] == 0 {
		pos = "…"
	} else {
		pos = fmt.Sprintf("%d/%d", m.nav.cur()+1, m.nav.counts[m.nav.level])
	}
	modeTag := "[NAV]"
	if m.mode == modeCmd {
		modeTag = "[CMD]"
	}
	return strings.Join(parts, " › ") + "   " + modeTag + " " + m.nav.level.String() + "   " + pos
}

// levelColumns returns the table columns for the current nav level, or nil for
// levels not rendered as a sortable table (Hosts / Inventory).
func (m model) levelColumns() []navColumn {
	switch m.nav.level {
	case levelPlayers:
		return []navColumn{
			{"NAME", cellLeft, true},
			{"STATUS", cellLeft, true},
		}
	case levelItem:
		return []navColumn{
			{"ID", cellLeft, true},
			{"NAME", cellLeft, true},
			{"STACK", cellRight, true},
			{"MAX", cellRight, false},
			{"Q", cellLeft, true},
		}
	}
	return nil
}

// sortLess reports whether row i sorts before row j at the current level's
// active sort column (ascending; the caller applies direction).
func (m model) sortLess(i, j int) bool {
	switch m.nav.level {
	case levelPlayers:
		a, b := m.players[i], m.players[j]
		switch m.sortCol[levelPlayers] {
		case 1: // STATUS
			return strings.ToLower(a.OnlineStatus) < strings.ToLower(b.OnlineStatus)
		default: // 0 NAME
			return strings.ToLower(a.CharacterName) < strings.ToLower(b.CharacterName)
		}
	case levelItem:
		a, b := m.items[i], m.items[j]
		switch m.sortCol[levelItem] {
		case 0: // ID
			return a.ID < b.ID
		case 2: // STACK
			return a.StackSize < b.StackSize
		case 4: // Q
			return a.Quality < b.Quality
		default: // 1 NAME
			return strings.ToLower(admincatalog.DisplayName(a.TemplateID)) <
				strings.ToLower(admincatalog.DisplayName(b.TemplateID))
		}
	}
	return false
}

// applySort stably reorders the current level's slice by the active column and
// direction. No-op for non-table levels. sortLess has a value receiver but
// reads through the slice header, which points to the same backing array
// sort.SliceStable is reordering, so it sees the in-progress swaps.
func (m *model) applySort() {
	level := m.nav.level
	less := func(i, j int) bool {
		if m.sortDesc[level] {
			return m.sortLess(j, i)
		}
		return m.sortLess(i, j)
	}
	switch level {
	case levelPlayers:
		sort.SliceStable(m.players, less)
	case levelItem:
		sort.SliceStable(m.items, less)
	}
}

// nextSortableCol returns the next sortable column index after cur (wrapping),
// or cur if none is sortable.
func nextSortableCol(cols []navColumn, cur int) int {
	n := len(cols)
	for k := 1; k <= n; k++ {
		idx := (cur + k) % n
		if cols[idx].sortable {
			return idx
		}
	}
	return cur
}

// levelCells returns the per-row cell text for the current table level, in the
// current (already-sorted) slice order.
func (m model) levelCells() [][]string {
	switch m.nav.level {
	case levelPlayers:
		out := make([][]string, len(m.players))
		for i, p := range m.players {
			out[i] = []string{p.CharacterName, p.OnlineStatus}
		}
		return out
	case levelItem:
		out := make([][]string, len(m.items))
		for i, it := range m.items {
			maxStr := ""
			if mx := admincatalog.StackMax(it.TemplateID); mx > 0 {
				maxStr = strconv.Itoa(mx)
			}
			out[i] = []string{
				fmt.Sprintf("id=%d", it.ID),
				admincatalog.DisplayName(it.TemplateID),
				strconv.FormatInt(it.StackSize, 10),
				maxStr,
				fmt.Sprintf("q%d", it.Quality),
			}
		}
		return out
	}
	return nil
}

func (m model) levelRows() []string {
	if cols := m.levelColumns(); cols != nil {
		_, rows := formatTable(cols, m.levelCells(), m.sortCol[m.nav.level], m.sortDesc[m.nav.level])
		return rows
	}
	switch m.nav.level {
	case levelHosts:
		return append([]string(nil), m.hosts...)
	case levelInventory:
		out := make([]string, len(m.invs))
		for i, inv := range m.invs {
			out[i] = fmt.Sprintf("%-12s %d items", command.InventoryTypeName(inv.InventoryType), inv.ItemCount)
		}
		return out
	}
	return nil
}

// levelHeader returns the pinned column header for the current nav level, or ""
// for levels without a table header (Hosts).
func (m model) levelHeader() string {
	if cols := m.levelColumns(); cols != nil {
		h, _ := formatTable(cols, m.levelCells(), m.sortCol[m.nav.level], m.sortDesc[m.nav.level])
		return h
	}
	if m.nav.level == levelInventory {
		return fmt.Sprintf("%-12s %s", "TYPE", "ITEMS")
	}
	return ""
}

// renderNavBody renders the nav list with its pinned level header (label + dim
// rule) above a scrolling data window of h-2 rows. When the level has no header
// the full height scrolls. For table levels the header is indented to match
// renderList's 2-col selection marker.
func (m model) renderNavBody(h int) string {
	header := m.levelHeader()
	rows := m.levelRows()
	if header == "" {
		return renderList(rows, m.nav.cur(), h)
	}
	const indent = "  " // matches renderList's 2-col selection marker
	rule := styleErr.Render(strings.Repeat("─", lipgloss.Width(indent+header)))
	dh := h - 2
	if dh < 1 {
		dh = 1
	}
	return indent + header + "\n" + rule + "\n" + renderList(rows, m.nav.cur(), dh)
}

// renderOutputBody renders the command-output pane: it pins the first
// outHeaderLines lines (label + dim rule) and scrolls only the data beneath
// them within h total rows.
func (m model) renderOutputBody(h int) string {
	if m.outHeaderLines <= 0 {
		return renderOutputScrolled(m.output, m.outScroll, h)
	}
	lines := strings.Split(strings.TrimRight(m.output, "\n"), "\n")
	n := m.outHeaderLines
	if n > len(lines) {
		n = len(lines)
	}
	header := strings.Join(lines[:n], "\n")
	rest := strings.Join(lines[n:], "\n")
	rule := styleErr.Render(strings.Repeat("─", lipgloss.Width(header)))
	dh := h - (n + 1)
	if dh < 1 {
		dh = 1
	}
	return header + "\n" + rule + "\n" + renderOutputScrolled(rest, m.outScroll, dh)
}

// outDataWindow is the number of scrolling data rows in the output pane (total
// content rows minus the pinned header + rule when present).
func (m model) outDataWindow() int {
	w := contentWindow(m.height)
	if m.outHeaderLines > 0 {
		w -= m.outHeaderLines + 1
	}
	if w < 1 {
		w = 1
	}
	return w
}

// legend returns the contextual key hints for the current nav level.
func (m model) legend() string {
	var keys string
	switch m.nav.level {
	case levelItem:
		keys = "↑↓ item  ← back  +/- qty  e qty  Q quality  d del  a add  s/S sort"
	case levelPlayers:
		keys = "↑↓ move  → in  ← back  g/G first/last  a add  s/S sort"
	default:
		keys = "↑↓ move  → in  ← back  g/G first/last  a add"
	}
	return keys + "\n[:] command   [?] help   [q] quit"
}

// currentSlotIsPlayer reports whether the in-progress token occupies an
// argPlayer slot, accounting for an implied selected host.
func currentSlotIsPlayer(line string, selHost string, hosts []string) bool {
	tokens, _ := splitCurrent(line)
	if len(tokens) == 0 {
		return false
	}
	spec, ok := command.SpecFor(tokens[0])
	if !ok {
		return false
	}
	norm := normalizeTokens(spec, tokens, selHost, hosts)
	return spec.IsPlayerPos(len(norm)-1, norm...)
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
				if h := m.selectedHost(); h != "" && currentSlotIsPlayer(m.input.Value(), h, m.hosts) {
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
						m.outHeaderLines = 0
						m.output = renderHelp(fields[1:])
						return m, nil
					case refreshVerb:
						h := m.selectedHost()
						delete(m.playerNames, h)
						m.outHeaderLines = 0
						if h != "" {
							m.output = "completion cache refreshed for " + h
						} else {
							m.output = "completion cache refreshed"
						}
						return m, nil
					}
				}
				m.running = true
				m.outHeaderLines = command.ListingHeaderLines(parseLine(injectHost(line, m.selectedHost(), m.hosts)))
				return m, tea.Batch(m.dispatch(line), m.spin.Tick)
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		// modeNav key handling — modal drill navigation.
		//
		// While a command result is showing, vertical keys scroll it and
		// left/esc dismisses it back to the nav list.
		if m.output != "" {
			switch msg.Type {
			case tea.KeyUp, tea.KeyPgUp:
				if m.outScroll > 0 {
					m.outScroll--
				}
				return m, nil
			case tea.KeyDown, tea.KeyPgDown:
				lines := len(strings.Split(strings.TrimRight(m.output, "\n"), "\n"))
				if m.outScroll < lines-m.outDataWindow() {
					m.outScroll++
				}
				return m, nil
			case tea.KeyLeft, tea.KeyEsc:
				m.output, m.navErr = "", ""
				return m, nil
			}
			switch msg.String() {
			case "k":
				if m.outScroll > 0 {
					m.outScroll--
				}
				return m, nil
			case "j":
				lines := len(strings.Split(strings.TrimRight(m.output, "\n"), "\n"))
				if m.outScroll < lines-m.outDataWindow() {
					m.outScroll++
				}
				return m, nil
			case "h":
				m.output, m.navErr = "", ""
				return m, nil
			case "q", "ctrl+c":
				return m, tea.Quit
			case ":":
				m.mode = modeCmd
				m.input.SetValue("")
				if hh := m.selectedHost(); hh != "" {
					m.input.Prompt = "[" + hh + "] › "
				} else {
					m.input.Prompt = "› "
				}
				return m, m.input.Focus()
			}
			return m, nil // swallow other keys while viewing output
		}
		// Confirm-delete sub-state: y commits, any other key cancels.
		if m.confirmDelete {
			if msg.String() == "y" {
				m.confirmDelete = false
				m.running = true
				return m, tea.Batch(m.applyDelete(m.selectedItemID()), m.spin.Tick)
			}
			m.confirmDelete = false // any other key cancels
			return m, nil
		}
		// Inline-edit sub-state: Enter commits, Esc cancels, everything else
		// feeds the text input.
		if m.editing {
			switch msg.Type {
			case tea.KeyEnter:
				v, err := strconv.ParseInt(strings.TrimSpace(m.input.Value()), 10, 64)
				id := m.selectedItemID()
				kind := m.editKind
				tmpl := ""
				if i := m.nav.sel[levelItem]; i >= 0 && i < len(m.items) {
					tmpl = m.items[i].TemplateID
				}
				m.editing = false
				m.input.Blur()
				m.input.SetValue("")
				if err != nil || v < 0 {
					m.navErr = "value must be a non-negative integer"
					return m, nil
				}
				if kind == editKindQty {
					if max := admincatalog.StackMax(tmpl); max > 0 && v > int64(max) {
						m.navErr = fmt.Sprintf("max stack is %d", max)
						return m, nil
					}
				}
				m.running = true
				if kind == editKindQuality {
					return m, tea.Batch(m.applyQuality(id, v), m.spin.Tick)
				}
				return m, tea.Batch(m.applyStack(id, v), m.spin.Tick)
			case tea.KeyEsc:
				m.editing = false
				m.input.Blur()
				m.input.SetValue("")
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}
		// Help overlay: any key dismisses it. This check runs in plain nav
		// only — it is placed after the output/confirmDelete/editing guards
		// above, so the overlay cannot open or close while those sub-states
		// are active.
		if m.showHelp {
			m.showHelp = false // any key dismisses
			return m, nil
		}
		switch msg.Type {
		case tea.KeyUp:
			m.nav.move(-1)
			return m, nil
		case tea.KeyDown:
			m.nav.move(1)
			return m, nil
		case tea.KeyPgUp:
			m.nav.move(-contentWindow(m.height))
			return m, nil
		case tea.KeyPgDown:
			m.nav.move(contentWindow(m.height))
			return m, nil
		case tea.KeyRight, tea.KeyEnter:
			return m.descend()
		case tea.KeyLeft, tea.KeyEsc:
			m.output, m.navErr = "", ""
			m.nav.ascend()
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "k":
			m.nav.move(-1)
			return m, nil
		case "j":
			m.nav.move(1)
			return m, nil
		case "h":
			m.output, m.navErr = "", ""
			m.nav.ascend()
			return m, nil
		case "l":
			return m.descend()
		case "g":
			m.nav.jump(false)
			return m, nil
		case "G":
			m.nav.jump(true)
			return m, nil
		case "s":
			if cols := m.levelColumns(); cols != nil {
				m.sortCol[m.nav.level] = nextSortableCol(cols, m.sortCol[m.nav.level])
				m.applySort()
				m.nav.sel[m.nav.level] = 0
			}
			return m, nil
		case "S":
			if m.levelColumns() != nil {
				m.sortDesc[m.nav.level] = !m.sortDesc[m.nav.level]
				m.applySort()
				m.nav.sel[m.nav.level] = 0
			}
			return m, nil
		case "?":
			m.showHelp = true
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
		case "a":
			player := m.curChar
			if m.nav.level == levelPlayers {
				if i := m.nav.sel[levelPlayers]; i >= 0 && i < len(m.players) {
					player = m.players[i].CharacterName
				}
			}
			if player == "" {
				m.navErr = "select a player first (drill into a host)"
				return m, nil
			}
			m.mode = modeCmd
			m.output = ""
			if h := m.selectedHost(); h != "" {
				m.input.Prompt = "[" + h + "] › "
			} else {
				m.input.Prompt = "› "
			}
			m.input.SetValue("give item " + player + " ")
			m.input.CursorEnd()
			return m, m.input.Focus()
		}
		// Item-level edit keys: only active at levelItem with at least one item.
		// These runes (+, -, =, e, Q, d) are distinct from all generic nav keys.
		if m.nav.level == levelItem && len(m.items) > 0 {
			cur := m.items[m.nav.sel[levelItem]]
			switch msg.String() {
			case "+", "=":
				max := admincatalog.StackMax(cur.TemplateID)
				if max > 0 && cur.StackSize >= int64(max) {
					m.navErr = fmt.Sprintf("already at max stack (%d)", max)
					return m, nil
				}
				m.running = true
				return m, tea.Batch(m.applyStack(cur.ID, cur.StackSize+1), m.spin.Tick)
			case "-":
				next := cur.StackSize - 1
				if next < 0 {
					next = 0
				}
				m.running = true
				return m, tea.Batch(m.applyStack(cur.ID, next), m.spin.Tick)
			case "e":
				m.editing, m.editKind = true, editKindQty
				m.input.Prompt = "qty: "
				m.input.SetValue("")
				return m, m.input.Focus()
			case "Q":
				m.editing, m.editKind = true, editKindQuality
				m.input.Prompt = "quality: "
				m.input.SetValue("")
				m.navErr = "quality_level is rarely used; effect undocumented"
				return m, m.input.Focus()
			case "d":
				m.confirmDelete = true
				return m, nil
			}
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
		switch {
		case msg.err == nil:
			m.output = msg.out
		case errors.Is(msg.err, command.ErrUsage):
			// The captured stderr already carries the helpful usage message;
			// don't append a redundant "error: … usage error" line.
			m.outHeaderLines = 0
			m.output = strings.TrimRight(msg.out, "\n")
		default:
			m.outHeaderLines = 0
			m.output = msg.out + "\nerror: " + msg.err.Error()
		}
		return m, nil
	case playerNamesMsg:
		m.playerNames[msg.host] = msg.names
		return m, nil
	case playersMsg:
		m.loading = false
		m.navErr = msg.err
		m.players = msg.players
		m.nav.counts[levelPlayers] = len(m.players)
		m.nav.sel[levelPlayers] = 0
		m.applySort()
		return m, nil
	case invsMsg:
		m.loading = false
		m.navErr = msg.err
		m.curFLS, m.curChar, m.invs = msg.fls, msg.char, msg.invs
		m.nav.counts[levelInventory] = len(m.invs)
		m.nav.sel[levelInventory] = 0
		return m, nil
	case itemsMsg:
		m.loading = false
		m.navErr = msg.err
		m.items = msg.items
		m.nav.counts[levelItem] = len(m.items)
		// Clamp the existing cursor to the new list length instead of forcing 0, so an
		// edit-reload keeps the operator near the previous position. (With an active
		// sort, the row at that index may differ once the list is re-sorted; the cursor
		// stays in-bounds.) descend() sets sel[levelItem]=0 on a fresh descent.
		if m.nav.sel[levelItem] > len(m.items)-1 {
			m.nav.sel[levelItem] = len(m.items) - 1
		}
		if m.nav.sel[levelItem] < 0 {
			m.nav.sel[levelItem] = 0
		}
		m.applySort()
		return m, nil
	case editDoneMsg:
		m.running = false
		switch {
		case errors.Is(msg.err, command.ErrItemOwnerOnline):
			m.navErr = "edit refused — owner online (use CLI: item … --force)"
		case errors.Is(msg.err, command.ErrItemOwnerUnknown):
			m.navErr = "edit refused — no resolvable player owner"
		case msg.err != nil:
			m.navErr = "edit failed: " + msg.err.Error()
		default:
			m.navErr = ""
			m.loading = true
			return m, m.loadItems() // refresh item list after successful mutation
		}
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
	header := renderHeader(m.hosts, m.statuses, m.nav.sel[levelHosts])

	// Build footer and footTitle first — they don't depend on content body.
	var footer string
	switch {
	case m.confirmDelete:
		footer = fmt.Sprintf("delete id=%d? (y/n)", m.selectedItemID())
	case m.editing:
		footer = m.input.View()
	case m.mode == modeCmd:
		footer = m.input.View()
		fw := m.width - 2
		if fw < 1 {
			fw = 1
		}
		if sugg := suggest(m.input.Value(), m.hosts, m.selectedHost(), m.playerNames); len(sugg) > 0 {
			footer += "\n" + styleErr.Render(renderSuggestions(sugg, fw, helperMaxRows))
		} else if hint := usageHint(m.input.Value()); hint != "" {
			footer += "\n" + styleErr.Render(wrapHelper(hint, fw, helperMaxRows))
		}
	case m.output != "":
		footer = "←/Esc back   ↑↓ scroll\n[:] command   [?] help   [q] quit"
	default:
		footer = m.legend()
	}
	if m.running {
		footer += "\n" + m.spin.View() + " running…"
	}

	footTitle := "Keys"
	if m.mode == modeCmd || m.editing {
		footTitle = "Command"
	}

	contentTitle := m.breadcrumb()
	if m.showHelp {
		contentTitle = "Help"
	}

	if m.width == 0 {
		h := contentWindow(m.height)
		var body string
		switch {
		case m.showHelp:
			body = helpText()
		case m.output != "":
			body = m.renderOutputBody(h)
		case m.loading:
			body = "loading…"
		default:
			body = m.renderNavBody(h)
		}
		if m.navErr != "" {
			body += "\n" + styleErr.Render(m.navErr)
		}
		return header + "\n" + contentTitle + "\n" + body + "\n" + footer
	}

	w := m.width - 2
	if w < 1 {
		w = 1
	}
	headerBox := framedBox("Hosts", header, w, 0)
	footTitleStr := footTitle
	if m.mode == modeCmd || m.editing {
		footTitleStr = styleAccent.Render(footTitle)
	}
	footerBox := framedBox(footTitleStr, footer, w, 0)
	contentH := m.height - lipgloss.Height(headerBox) - lipgloss.Height(footerBox)
	if contentH < 3 {
		contentH = 3
	}
	h := contentH - 2 // inner rows (box has a top+bottom border)
	if h < 1 {
		h = 1
	}
	var body string
	switch {
	case m.showHelp:
		body = helpText()
	case m.output != "":
		body = m.renderOutputBody(h)
	case m.loading:
		body = "loading…"
	default:
		body = m.renderNavBody(h)
	}
	if m.navErr != "" {
		body += "\n" + styleErr.Render(m.navErr)
	}
	contentBox := framedBox(contentTitle, body, w, contentH)
	return lipgloss.JoinVertical(lipgloss.Left, headerBox, contentBox, footerBox)
}

// contentWindow is how many content lines to show given terminal height
// (header + footer + borders take ~8 rows).
func contentWindow(height int) int {
	w := height - 8
	if w < 3 {
		w = 3
	}
	return w
}
