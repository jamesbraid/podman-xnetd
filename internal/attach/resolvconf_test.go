// SPDX-License-Identifier: Apache-2.0
package attach

import (
	"net"
	"strings"
	"testing"

	"go.podman.io/common/libnetwork/types"

	"github.com/jamesbraid/podman-xnetd/internal/proto"
)

func TestRenderResolvConfDedupAndOrder(t *testing.T) {
	status := map[string]types.StatusBlock{
		"net1": {DNSServerIPs: []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("fd00::1")}},
		"net2": {DNSServerIPs: []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2")}},
	}
	out := string(renderResolvConf(status))
	for _, want := range []string{"nameserver 10.0.0.1", "nameserver fd00::1", "nameserver 10.0.0.2"} {
		if strings.Count(out, want) != 1 {
			t.Fatalf("want one %q in\n%s", want, out)
		}
	}
	if !strings.Contains(out, "search dns.podman") {
		t.Fatalf("missing search:\n%s", out)
	}
	if strings.Index(out, "10.0.0.1") > strings.Index(out, "10.0.0.2") {
		t.Fatal("want first-seen order")
	}
}

func TestWriteResolvConfGuardNoop(t *testing.T) {
	for _, req := range []proto.Request{{}, {RootfsPath: "/x"}, {Pid: 5}} {
		if err := WriteResolvConf(req, nil); err != nil {
			t.Fatalf("guarded no-op should be nil, got %v", err)
		}
	}
}
