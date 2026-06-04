package tui

// navLevel is one drill level in the modal navigation.
type navLevel int

const (
	levelHosts navLevel = iota
	levelPlayers
	levelInventory
	levelItem
)

func (l navLevel) String() string {
	switch l {
	case levelHosts:
		return "HOSTS"
	case levelPlayers:
		return "PLAYERS"
	case levelInventory:
		return "INVENTORY"
	case levelItem:
		return "ITEM"
	}
	return "?"
}

// navState is the cursor/level state of the modal navigation. The loaded list
// data lives on the model; navState holds only the current level, the per-level
// selection index, and the per-level item counts (so move/descend can clamp).
type navState struct {
	level  navLevel
	sel    [4]int
	counts [4]int
}

// move shifts the selection in the current level by delta, clamped to [0, counts-1].
func (n *navState) move(delta int) {
	c := n.counts[n.level]
	if c == 0 {
		n.sel[n.level] = 0
		return
	}
	s := n.sel[n.level] + delta
	if s < 0 {
		s = 0
	}
	if s > c-1 {
		s = c - 1
	}
	n.sel[n.level] = s
}

// jump sets the selection to first (0) or last (counts-1) of the current level.
func (n *navState) jump(toEnd bool) {
	if !toEnd {
		n.sel[n.level] = 0
		return
	}
	s := n.counts[n.level] - 1
	if s < 0 {
		s = 0
	}
	n.sel[n.level] = s
}

// descend moves one level deeper (clamped at levelItem), resetting the deeper
// level's selection. Returns true if the level changed.
func (n *navState) descend() bool {
	if n.level >= levelItem {
		return false
	}
	n.level++
	n.sel[n.level] = 0
	return true
}

// ascend moves one level shallower (clamped at levelHosts). Returns true if the
// level changed.
func (n *navState) ascend() bool {
	if n.level <= levelHosts {
		return false
	}
	n.level--
	return true
}

// cur returns the current selection index of the current level.
func (n *navState) cur() int { return n.sel[n.level] }

// visibleWindow returns [start,end) bounds over a list of n items so the
// selected index stays within a window of the given height (cursor-follow
// scrolling). end-start <= height; selected is always in [start,end).
func visibleWindow(n, selected, height int) (start, end int) {
	if height < 1 {
		height = 1
	}
	if n <= height {
		return 0, n
	}
	start = selected - (height - 1)
	if start < 0 {
		start = 0
	}
	end = start + height
	if end > n {
		end = n
		start = end - height
	}
	return start, end
}
