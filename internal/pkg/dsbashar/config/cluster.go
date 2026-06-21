package config

import (
	"strconv"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/clusterconfig"
)

const (
	ConfigMapName   = "dapdsm-bg-config"
	ConfigNamespace = "dapdsm-system"
)

// ToData projects a Config (plus secrets) onto the generic clusterconfig record.
func ToData(c Config, flsToken, serverPassword []byte) clusterconfig.Data {
	v := map[string]string{
		"TARGET":       string(c.Target),
		"WORLD_NAME":   c.WorldName,
		"WORLD_REGION": c.WorldRegion,
	}
	if c.GamePortBase > 0 {
		v["GAME_PORT_BASE"] = strconv.Itoa(c.GamePortBase)
	}
	if c.IGWPortBase > 0 {
		v["IGW_PORT_BASE"] = strconv.Itoa(c.IGWPortBase)
	}
	if len(c.AlwaysOnSets) > 0 {
		v["ALWAYS_ON_SETS"] = strings.Join(c.AlwaysOnSets, " ")
	}
	if c.ServerDisplayName != "" {
		v["SERVER_DISPLAY_NAME"] = c.ServerDisplayName
	}
	if c.HostDatacenterID != "" {
		v["HOST_DATACENTER_ID"] = c.HostDatacenterID
	}
	sec := map[string][]byte{}
	if len(flsToken) > 0 {
		sec["fls-token"] = flsToken
	}
	if len(serverPassword) > 0 {
		sec["server-password"] = serverPassword
	}
	return clusterconfig.Data{Values: v, Secrets: sec}
}

// FromData reconstructs a Config from a clusterconfig record (secrets are
// materialised to files by the caller; only Values map to Config fields).
func FromData(d clusterconfig.Data) Config {
	c := Config{
		Target:            Target(d.Values["TARGET"]),
		WorldName:         d.Values["WORLD_NAME"],
		WorldRegion:       d.Values["WORLD_REGION"],
		ServerDisplayName: d.Values["SERVER_DISPLAY_NAME"],
		HostDatacenterID:  d.Values["HOST_DATACENTER_ID"],
	}
	if n, err := strconv.Atoi(d.Values["GAME_PORT_BASE"]); err == nil {
		c.GamePortBase = n
	}
	if n, err := strconv.Atoi(d.Values["IGW_PORT_BASE"]); err == nil {
		c.IGWPortBase = n
	}
	if s := d.Values["ALWAYS_ON_SETS"]; s != "" {
		c.AlwaysOnSets = strings.Fields(s)
	}
	c.applyDefaults()
	return c
}

// Override carries the bring-up CLI flags (empty = unset).
type Override struct {
	WorldName, WorldRegion, ServerDisplayName string
}

// Merge returns base with any non-empty Override field applied (flags win).
func Merge(base Config, o Override) Config {
	if o.WorldName != "" {
		base.WorldName = o.WorldName
	}
	if o.WorldRegion != "" {
		base.WorldRegion = o.WorldRegion
	}
	if o.ServerDisplayName != "" {
		base.ServerDisplayName = o.ServerDisplayName
	}
	return base
}
