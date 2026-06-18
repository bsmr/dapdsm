package bootstrap

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderBanner_FiveAlignedLines(t *testing.T) {
	lines := RenderBanner()
	if len(lines) != 5 {
		t.Fatalf("want 5 lines, got %d", len(lines))
	}
	w := len([]rune(lines[0]))
	for i, l := range lines {
		if len([]rune(l)) != w {
			t.Fatalf("line %d width %d != %d", i, len([]rune(l)), w)
		}
	}
}

func TestPrintBanner_NoAnsiWhenNotAnimated(t *testing.T) {
	var buf bytes.Buffer
	PrintBanner(&buf, false)
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("escape codes leaked into non-animated output")
	}
}
