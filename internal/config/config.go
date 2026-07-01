// SPDX-License-Identifier: Apache-2.0

// Package config loads the xnetd TOML config with strict decoding.
package config

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	AllowedUsers []string         `toml:"allowed_users"`
	Paths        PathsConfig      `toml:"paths"`
	Libnetwork   LibnetworkConfig `toml:"libnetwork"`
	Runtime      RuntimeConfig    `toml:"runtime"`
}
type PathsConfig struct {
	Netavark string `toml:"netavark"`
	Aardvark string `toml:"aardvark"`
}
type LibnetworkConfig struct {
	NetworkConfigDir string `toml:"network_config_dir"`
	NetworkRunDir    string `toml:"network_run_dir"`
}
type RuntimeConfig struct {
	StateDir string `toml:"state_dir"`
	Socket   string `toml:"socket"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := toml.NewDecoder(f)
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		if sme, ok := err.(*toml.StrictMissingError); ok {
			var msg string
			for _, e := range sme.Errors {
				key := e.Key()
				if len(key) > 0 {
					if msg != "" {
						msg += ", "
					}
					msg += key[0]
				}
			}
			if msg != "" {
				return nil, fmt.Errorf("config %s: unknown fields: %s", path, msg)
			}
		}
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &c, nil
}

// ResolveAllowedUIDs resolves AllowedUsers to a uid set once at startup.
func (c *Config) ResolveAllowedUIDs() (map[int]struct{}, error) {
	out := make(map[int]struct{}, len(c.AllowedUsers))
	for _, name := range c.AllowedUsers {
		u, err := user.Lookup(name)
		if err != nil {
			return nil, fmt.Errorf("resolve allowed user %q: %w", name, err)
		}
		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			return nil, fmt.Errorf("parse uid for %q: %w", name, err)
		}
		out[uid] = struct{}{}
	}
	return out, nil
}
