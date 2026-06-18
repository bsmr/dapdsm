package bootstrap

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// 5-row block letters, fixed width per letter so columns stay aligned.
var glyphs = map[rune][]string{
	'A': {" ████ ", "██  ██", "██████", "██  ██", "██  ██"},
	'R': {"█████ ", "██  ██", "█████ ", "██ ██ ", "██  ██"},
	'K': {"██  ██", "██ ██ ", "████  ", "██ ██ ", "██  ██"},
	'I': {"████", " ██ ", " ██ ", " ██ ", "████"},
	'S': {" █████", "██    ", " ████ ", "    ██", "█████ "},
}

// RenderBanner returns the 5 assembled ASCII lines for "ARRAKIS".
func RenderBanner() []string {
	lines := make([]string, 5)
	word := "ARRAKIS"
	for i := range lines {
		var parts []string
		for _, r := range word {
			parts = append(parts, glyphs[r][i])
		}
		lines[i] = strings.Join(parts, " ")
	}
	return lines
}

// spice returns RGB values for the spice gradient (deep amber -> orange -> pale sand).
// t is expected to be in [0, 1].
func spice(t float64) (int, int, int) {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	// two-stop lerp through orange.
	r := int(0x8B + t*(0xF4-0x8B))
	g := int(0x45 + t*(0xE4-0x45))
	b := int(0x13 + t*(0xBC-0x13))
	return r, g, b
}

// sineish returns a cheap triangle wave in [-1,1], avoiding math import.
func sineish(x float64) float64 {
	x = x - float64(int(x)) // frac
	if x < 0.5 {
		return -1 + 4*x
	}
	return 3 - 4*x
}

// draw writes the lines to w, optionally with color animation.
// If color is true, applies the spice gradient with a moving highlight band.
// offset controls the phase of the animation (expected in [0, 24) for 24 frames).
func draw(w io.Writer, lines []string, offset float64, color bool) {
	for _, line := range lines {
		runes := []rune(line)
		var sb strings.Builder
		for i, ch := range runes {
			if color && ch != ' ' {
				// moving highlight band: brightness peaks near offset.
				phase := float64(i)/float64(len(runes)) + offset
				t := 0.5 + 0.5*sineish(phase)
				r, g, b := spice(t)
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%c", r, g, b, ch)
			} else {
				sb.WriteRune(ch)
			}
		}
		if color {
			sb.WriteString("\x1b[0m")
		}
		fmt.Fprintln(w, sb.String())
	}
}

// PrintBanner writes the ARRAKIS banner to w.
// If animate is false, prints the 5 lines once without color or escape codes.
// If animate is true, runs a 24-frame colored animation (80ms per frame, spice gradient),
// hiding/restoring the cursor with ANSI codes.
// In both cases, appends the backronym line.
func PrintBanner(w io.Writer, animate bool) {
	lines := RenderBanner()

	if !animate {
		draw(w, lines, 0, false)
		fmt.Fprintln(w, "\n  Automated Rancher Runtime And Kubernetes Init System")
		return
	}

	// animated mode: hide cursor, loop 24 frames, restore cursor.
	fmt.Fprint(w, "\x1b[?25l")       // hide cursor
	defer fmt.Fprint(w, "\x1b[?25h") // restore
	for frame := 0; frame < 24; frame++ {
		if frame > 0 {
			fmt.Fprintf(w, "\x1b[%dA", len(lines)) // cursor up to redraw
		}
		draw(w, lines, float64(frame)/8.0, true)
		time.Sleep(80 * time.Millisecond)
	}
	fmt.Fprintln(w, "\n  Automated Rancher Runtime And Kubernetes Init System")
}
