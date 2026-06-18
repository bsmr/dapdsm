package kubedist

import "io"

// newK3s returns a Distro backed by k3s, using the shared rancherDistro base.
func newK3s(r Runner, stdout io.Writer) *rancherDistro {
	return &rancherDistro{
		name:         "k3s",
		configDir:    "/etc/rancher/k3s",
		dropinDir:    "/etc/rancher/k3s/config.yaml.d",
		imagesDir:    "/var/lib/rancher/k3s/agent/images",
		kubeconfig:   "/etc/rancher/k3s/k3s.yaml",
		service:      "k3s",
		installerURL: "https://get.k3s.io",
		r:            r,
		stdout:       stdout,
	}
}
