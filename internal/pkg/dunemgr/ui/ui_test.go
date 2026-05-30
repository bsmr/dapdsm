package ui

import (
	"bytes"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

func TestRenderLogin(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	if err := r.Render(&buf, "login", nil); err != nil {
		t.Fatalf("Render login: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`<title>dunemgr ` + "—" + ` login</title>`, `name="token"`, `action="/login"`} {
		if !strings.Contains(out, want) {
			t.Errorf("login output missing %q", want)
		}
	}
	if strings.Contains(out, `hx-get="/hosts"`) {
		t.Error("login output must not contain sidebar nav")
	}
}

func TestRenderHomeUsesOperatorName(t *testing.T) {
	r := New()
	data := struct{ Operator struct{ Name string } }{}
	data.Operator.Name = "Alice"
	var buf bytes.Buffer
	if err := r.Render(&buf, "home", data); err != nil {
		t.Fatalf("Render home: %v", err)
	}
	if !strings.Contains(buf.String(), "Hello, Alice") {
		t.Errorf("home output missing greeting: %s", buf.String())
	}
}

func TestStaticFSHasAssets(t *testing.T) {
	fs := StaticFS()
	for _, name := range []string{"pico.min.css", "htmx.min.js", "app.css", "app.js"} {
		f, err := fs.Open(name)
		if err != nil {
			t.Errorf("open %s: %v", name, err)
			continue
		}
		_ = f.Close()
	}
}

func TestRenderSidebar(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	data := struct {
		Hosts []struct {
			Name    string
			BGState string
		}
	}{
		Hosts: []struct {
			Name    string
			BGState string
		}{
			{Name: "vm-a", BGState: "RUNNING"},
			{Name: "vm-b", BGState: "STOPPED"},
		},
	}
	if err := r.RenderPartial(&buf, "sidebar", data); err != nil {
		t.Fatalf("RenderPartial sidebar: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"vm-a", "vm-b", "RUNNING", "STOPPED"} {
		if !strings.Contains(out, want) {
			t.Errorf("sidebar output missing %q", want)
		}
	}
}

func TestLayoutThemeToggleIsDiscreet(t *testing.T) {
	r := New()
	data := struct{ Operator struct{ Name string } }{}
	data.Operator.Name = "Alice"
	var buf bytes.Buffer
	if err := r.Render(&buf, "home", data); err != nil {
		t.Fatalf("Render home: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, `id="theme-toggle"`) || !strings.Contains(body, `aria-label="toggle dark mode"`) {
		t.Errorf("theme toggle not discreet/labelled: %s", body)
	}
	if strings.Contains(body, "◐ theme") {
		t.Error("old oversized theme button still present")
	}
}

func TestRenderDashboard(t *testing.T) {
	r := New()
	data := struct {
		Host       string
		BG         string
		Snap       store.StatusSnapshot
		LastAction *store.AuditEntry
	}{Host: "vm-a", BG: "vm-a"}
	data.Snap.BGState = "RUNNING"
	data.Snap.PodReady = 14
	data.Snap.PodTotal = 14
	var buf bytes.Buffer
	if err := r.Render(&buf, "dashboard", data); err != nil {
		t.Fatalf("Render dashboard: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"vm-a", "RUNNING", "14/14"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard missing %q in %q", want, out)
		}
	}
}
