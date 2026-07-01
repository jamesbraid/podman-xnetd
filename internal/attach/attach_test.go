// SPDX-License-Identifier: Apache-2.0
package attach

import (
	"testing"

	"github.com/jamesbraid/xnetd/internal/config"
)

func testCfg() *config.Config {
	return &config.Config{
		Paths:      config.PathsConfig{Netavark: "/usr/lib/podman/netavark", Aardvark: "/usr/lib/podman/aardvark-dns"},
		Libnetwork: config.LibnetworkConfig{NetworkConfigDir: "/etc/containers/networks", NetworkRunDir: "/run/containers/networks"},
		Runtime:    config.RuntimeConfig{StateDir: "/run/xnetd", Socket: "/run/xnetd/sock"},
	}
}

func TestNew(t *testing.T) {
	a, err := New(testCfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a == nil || a.iface == nil || a.cfg == nil {
		t.Fatal("New returned incomplete Attacher")
	}
}

func TestNewNilConfig(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) should error")
	}
}

func TestNetnsPath(t *testing.T) {
	a, _ := New(testCfg())
	if got := a.netnsPath("abc123"); got != "/run/xnetd/netns/abc123" {
		t.Fatalf("netnsPath = %q", got)
	}
}

func TestIfaceName(t *testing.T) {
	for i, want := range map[int]string{0: "eth0", 1: "eth1", 7: "eth7"} {
		if ifaceName(i) != want {
			t.Fatalf("ifaceName(%d) != %q", i, want)
		}
	}
}
