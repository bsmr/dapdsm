// rke2 image-import path + ctr command are UNVERIFIED on a real host (spec §11).
package kubedist

import "io"

// newRKE2 returns a Distro backed by rke2, using the shared rancherDistro base.
func newRKE2(r Runner, stdout io.Writer) *rancherDistro {
	return &rancherDistro{
		name:         "rke2",
		configDir:    "/etc/rancher/rke2",
		dropinDir:    "/etc/rancher/rke2/config.yaml.d",
		imagesDir:    "/var/lib/rancher/rke2/agent/images",
		kubeconfig:   "/etc/rancher/rke2/rke2.yaml",
		service:      "rke2-server",
		installerURL: "https://get.rke2.io",
		r:            r,
		stdout:       stdout,
	}
}
