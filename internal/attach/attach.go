// SPDX-License-Identifier: Apache-2.0

// Package attach wires container netns to podman networks via libnetwork.
package attach

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"

	"go.podman.io/common/libnetwork/netavark"
	"go.podman.io/common/libnetwork/types"
	pconfig "go.podman.io/common/pkg/config"

	"github.com/jamesbraid/xnetd/internal/config"
	"github.com/jamesbraid/xnetd/internal/proto"
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

func (a *Attacher) netnsPath(cid string) string {
	return filepath.Join(a.cfg.Runtime.StateDir, "netns", cid)
}

func ifaceName(i int) string { return fmt.Sprintf("eth%d", i) }

func parseStaticIPs(ss []string) ([]net.IP, error) {
	if len(ss) == 0 {
		return nil, nil
	}
	out := make([]net.IP, 0, len(ss))
	for _, s := range ss {
		ip := net.ParseIP(s)
		if ip == nil {
			return nil, fmt.Errorf("attach: invalid static IP %q", s)
		}
		out = append(out, ip)
	}
	return out, nil
}

func (a *Attacher) buildNetworkOptions(req proto.Request) (types.NetworkOptions, error) {
	nets := make(map[string]types.PerNetworkOptions, len(req.Networks))
	for i, name := range req.Networks {
		ips, err := parseStaticIPs(req.StaticIPs[name])
		if err != nil {
			return types.NetworkOptions{}, err
		}
		nets[name] = types.PerNetworkOptions{InterfaceName: ifaceName(i), StaticIPs: ips}
	}
	return types.NetworkOptions{ContainerID: req.ContainerID, ContainerName: req.ContainerName, Networks: nets}, nil
}
