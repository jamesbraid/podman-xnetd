// SPDX-License-Identifier: Apache-2.0

// Package attach wires container netns to podman networks via libnetwork.
package attach

import (
	"errors"

	"go.podman.io/common/libnetwork/netavark"
	"go.podman.io/common/libnetwork/types"
	pconfig "go.podman.io/common/pkg/config"

	"github.com/jamesbraid/xnetd/internal/config"
)

// Attacher wires container network namespaces to rootful networks via the
// netavark backend. cfg is xnetd's own config (StateDir + binary paths).
type Attacher struct {
	iface types.ContainerNetwork
	cfg   *config.Config
}

// New builds a netavark-backed Attacher. It constructs podman's own config
// (config.Default) for the library and sources dirs/binaries from cfg.
func New(cfg *config.Config) (*Attacher, error) {
	if cfg == nil {
		return nil, errors.New("attach: nil config")
	}
	pcfg, err := pconfig.Default()
	if err != nil {
		return nil, err
	}
	iface, err := netavark.NewNetworkInterface(&netavark.InitConfig{
		Config:           pcfg,
		NetworkConfigDir: cfg.Libnetwork.NetworkConfigDir,
		NetworkRunDir:    cfg.Libnetwork.NetworkRunDir,
		NetavarkBinary:   cfg.Paths.Netavark,
		AardvarkBinary:   cfg.Paths.Aardvark,
	})
	if err != nil {
		return nil, err
	}
	return &Attacher{iface: iface, cfg: cfg}, nil
}
