package kubedist

import (
	"bytes"
	"fmt"
	"path"
	"text/template"
)

// baseConfig returns the embedded baseline config.yaml for the given distro.
func baseConfig(distro string) ([]byte, error) {
	return assets.ReadFile(path.Join("embed", distro, "config.yaml"))
}

// renderDropins renders the drop-in YAML files for the given distro and Config.
// It always returns 10-host.yaml and 20-external-ip.yaml.
// 30-internal-routing.yaml is included only when InternalIP is non-empty and
// differs from ExternalIP (i.e. the node is behind DNAT).
func renderDropins(distro string, cfg Config) (map[string][]byte, error) {
	files := []string{"10-host.yaml", "20-external-ip.yaml"}
	if cfg.InternalIP != "" && cfg.InternalIP != cfg.ExternalIP {
		files = append(files, "30-internal-routing.yaml")
	}
	out := make(map[string][]byte, len(files))
	for _, name := range files {
		tmplBytes, err := assets.ReadFile(path.Join("embed", distro, name+".tmpl"))
		if err != nil {
			return nil, fmt.Errorf("read template %s/%s: %w", distro, name, err)
		}
		t, err := template.New(name).Parse(string(tmplBytes))
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, cfg); err != nil {
			return nil, fmt.Errorf("execute template %s: %w", name, err)
		}
		out[name] = buf.Bytes()
	}
	return out, nil
}
