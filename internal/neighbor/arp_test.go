// SPDX-License-Identifier: Apache-2.0
package neighbor

import (
	"bytes"
	"net"
	"testing"
)

func TestBuildGratuitousARP(t *testing.T) {
	ip := net.ParseIP("10.0.0.5").To4()
	mac, _ := net.ParseMAC("02:42:0a:00:00:05")
	p := buildGratuitousARP(ip, mac)
	if len(p) != 28 {
		t.Fatalf("len=%d want 28", len(p))
	}
	if p[0] != 0 || p[1] != 1 || p[2] != 8 || p[3] != 0 || p[4] != 6 || p[5] != 4 || p[6] != 0 || p[7] != 1 {
		t.Fatalf("header %x", p[:8])
	}
	if !bytes.Equal(p[8:14], mac) || !bytes.Equal(p[14:18], ip) ||
		!bytes.Equal(p[18:24], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) || !bytes.Equal(p[24:28], ip) {
		t.Fatalf("body %x", p[8:])
	}
}
