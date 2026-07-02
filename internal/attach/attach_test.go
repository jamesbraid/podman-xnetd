// SPDX-License-Identifier: Apache-2.0
package attach

import (
	"net"
	"os"
	"testing"

	"github.com/jamesbraid/podman-xnetd/internal/config"
	"github.com/jamesbraid/podman-xnetd/internal/proto"
)

func testCfg() *config.Config {
	return &config.Config{
		Paths:      config.PathsConfig{Netavark: "/usr/lib/podman/netavark", Aardvark: "/usr/lib/podman/aardvark-dns"},
		Libnetwork: config.LibnetworkConfig{NetworkConfigDir: "/etc/containers/networks", NetworkRunDir: "/run/containers/networks"},
		Runtime:    config.RuntimeConfig{StateDir: "/run/xnetd", Socket: "/run/xnetd/sock"},
	}
}

func TestNew(t *testing.T) {
	// New constructs libnetwork's netavark backend, which opens the netavark
	// lock under /run/lock — that needs root. The full construction is
	// exercised by the integration harness; skip when not privileged.
	if os.Geteuid() != 0 {
		t.Skip("New opens the netavark lock (needs root); covered by the integration harness")
	}
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
	// netnsPath only needs cfg; build the Attacher directly so this stays a
	// pure unit test (no privileged libnetwork construction).
	a := &Attacher{cfg: testCfg()}
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

func TestParseStaticIPs(t *testing.T) {
	got, err := parseStaticIPs([]string{"10.0.0.5", "fd00::5"})
	if err != nil || len(got) != 2 || !got[0].Equal(net.ParseIP("10.0.0.5")) || !got[1].Equal(net.ParseIP("fd00::5")) {
		t.Fatalf("got %v err %v", got, err)
	}
	if _, err := parseStaticIPs([]string{"nope"}); err == nil {
		t.Fatal("bad ip should error")
	}
	if got, _ := parseStaticIPs(nil); len(got) != 0 {
		t.Fatal("nil should be empty")
	}
}

func TestBuildNetworkOptions(t *testing.T) {
	a := &Attacher{}
	opts, err := a.buildNetworkOptions(proto.Request{
		ContainerID: "cid", ContainerName: "cname",
		Networks:  []string{"net1", "net2"},
		StaticIPs: map[string][]string{"net2": {"10.0.1.9"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.ContainerID != "cid" || opts.Networks["net1"].InterfaceName != "eth0" ||
		opts.Networks["net2"].InterfaceName != "eth1" || len(opts.Networks["net1"].StaticIPs) != 0 ||
		len(opts.Networks["net2"].StaticIPs) != 1 {
		t.Fatalf("opts = %+v", opts)
	}
}

func TestAttachRejectsEmptyContainerID(t *testing.T) {
	if _, err := (&Attacher{}).Attach(proto.Request{Networks: []string{"n"}}); err == nil {
		t.Fatal("want error")
	}
}

func TestAttachRejectsNoNetworks(t *testing.T) {
	if _, err := (&Attacher{}).Attach(proto.Request{ContainerID: "cid"}); err == nil {
		t.Fatal("want error")
	}
}

func TestDetachRejectsEmptyContainerID(t *testing.T) {
	if err := (&Attacher{}).Detach(proto.Request{Networks: []string{"n"}}); err == nil {
		t.Fatal("want error")
	}
}

func TestAttachBadStaticIPBeforeSetup(t *testing.T) {
	_, err := (&Attacher{}).Attach(proto.Request{ContainerID: "cid", Networks: []string{"n"}, StaticIPs: map[string][]string{"n": {"garbage"}}})
	if err == nil {
		t.Fatal("bad static IP must error before Setup")
	}
}
