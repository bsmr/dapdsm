package kubedist

import (
	"fmt"
	"io"
)

func New(name string, r Runner, stdout io.Writer) (Distro, error) {
	switch name {
	case "k3s":
		return newK3s(r, stdout), nil
	case "rke2":
		return newRKE2(r, stdout), nil
	default:
		return nil, fmt.Errorf("unknown distro %q (want k3s|rke2)", name)
	}
}
