// SPDX-License-Identifier: Apache-2.0
package neighbor

import (
	"net"
	"testing"
)

func TestAnnounceEmptyNoop(t *testing.T) {
	if err := Announce("/nonexistent", "eth0", nil, net.HardwareAddr{1, 2, 3, 4, 5, 6}); err != nil {
		t.Fatalf("empty addrs should no-op: %v", err)
	}
}
func TestBuildFor(t *testing.T) {
	mac, _ := net.ParseMAC("02:42:0a:00:00:05")
	if p, v6 := buildFor(net.ParseIP("10.0.0.5"), mac); v6 || len(p) != 28 {
		t.Fatalf("v4 wrong: v6=%v len=%d", v6, len(p))
	}
	if p, v6 := buildFor(net.ParseIP("fd00::5"), mac); !v6 || len(p) != 32 {
		t.Fatalf("v6 wrong: v6=%v len=%d", v6, len(p))
	}
}
func TestAnnounceBadNetnsErrors(t *testing.T) {
	if err := Announce("/nonexistent/netns", "eth0", []net.IP{net.ParseIP("10.0.0.5")}, net.HardwareAddr{1, 2, 3, 4, 5, 6}); err == nil {
		t.Fatal("bad netns should error")
	}
}
