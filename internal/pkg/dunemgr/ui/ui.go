// Package ui ships embedded HTML templates and static assets.
package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Renderer renders templates by name. Each named template is
// parsed alongside layout.html so {{block}} overrides work.
type Renderer struct {
	templates map[string]*template.Template
}

// pageFiles defines which template files define {{define "body"}} / {{define
// "title"}} — i.e. full-page templates rendered via the layout wrapper.
// Partials (sidebar, lifecycle, backups) are included alongside every page so
// {{template "lifecycle" .}} etc. resolve, but they only define their own
// unique named blocks and thus do not conflict with any page's body/title.
var pageFiles = []string{
	"templates/dashboard.html",
	"templates/home.html",
	"templates/hostadd.html",
	"templates/login.html",
}

// New parses every page template alongside layout.html and all partials,
// returning a ready Renderer.  Each page gets its own template.Template set
// so its {{define "body"}} / {{define "title"}} are authoritative.
func New() *Renderer {
	r := &Renderer{templates: map[string]*template.Template{}}

	// Collect partial files — everything that is not layout and not a page.
	all, err := fs.Glob(templatesFS, "templates/*.html")
	if err != nil {
		panic(fmt.Errorf("glob templates: %w", err))
	}
	pageSet := make(map[string]bool, len(pageFiles))
	for _, p := range pageFiles {
		pageSet[p] = true
	}
	var partials []string
	for _, p := range all {
		if p == "templates/layout.html" || pageSet[p] {
			continue
		}
		partials = append(partials, p)
	}

	for _, p := range pageFiles {
		// layout + page + all partials — unique {{define}} names so no conflict.
		files := append([]string{"templates/layout.html", p}, partials...)
		t := template.Must(template.ParseFS(templatesFS, files...))
		name := p[len("templates/") : len(p)-len(".html")]
		r.templates[name] = t
	}
	return r
}

// Render writes the named template's output, using "layout" as the
// entry point.
func (r *Renderer) Render(w io.Writer, name string, data any) error {
	t, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("ui: unknown template %q", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

// StaticFS returns an fs.FS rooted at the static/ subtree, ready
// to hand to http.FileServer.
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

// RenderPartial writes the named template directly (no layout
// wrap). Used for HTMX swap-in fragments.
func (r *Renderer) RenderPartial(w io.Writer, name string, data any) error {
	// Parse all templates into one set so any partial can be looked
	// up by its {{define}} name.
	tset, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	if t := tset.Lookup(name); t != nil {
		return t.Execute(w, data)
	}
	return fmt.Errorf("ui: unknown partial %q", name)
}
