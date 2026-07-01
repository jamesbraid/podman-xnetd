// SPDX-License-Identifier: Apache-2.0
package neighbor

import (
	"bytes"
	"net"
	"testing"
)

func TestBuildUnsolicitedNA(t *testing.T) {
	ip := net.ParseIP("fd00::5")
	mac, _ := net.ParseMAC("02:42:0a:00:00:05")
	b := buildUnsolicitedNA(ip, mac)
	if len(b) != 32 || b[0] != 136 || b[1] != 0 || b[2] != 0 || b[3] != 0 || b[4] != 0x20 {
		t.Fatalf("head %x", b[:8])
	}
	if !bytes.Equal(b[8:24], ip.To16()) || b[24] != 2 || b[25] != 1 || !bytes.Equal(b[26:32], mac) {
		t.Fatalf("body %x", b[8:])
	}
	if b[4]&0x80 != 0 || b[4]&0x40 != 0 || b[4]&0x20 == 0 {
		t.Fatal("flags must be R0 S0 Override1")
	}
}
