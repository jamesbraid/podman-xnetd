// SPDX-License-Identifier: Apache-2.0
package auth

import (
	"net"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func socketPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	fds, _ := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	mk := func(fd int) *net.UnixConn {
		f := os.NewFile(uintptr(fd), "sock")
		c, _ := net.FileConn(f)
		f.Close()
		return c.(*net.UnixConn)
	}
	return mk(fds[0]), mk(fds[1])
}

func TestPeerUIDIsCurrentProcess(t *testing.T) {
	a, b := socketPair(t)
	defer a.Close()
	defer b.Close()
	got, err := PeerUID(b)
	if err != nil {
		t.Fatalf("PeerUID: %v", err)
	}
	if got != os.Getuid() {
		t.Fatalf("PeerUID = %d, want %d", got, os.Getuid())
	}
}

func TestAllowed(t *testing.T) {
	set := map[int]struct{}{1000: {}, 1001: {}}
	for uid, want := range map[int]bool{1000: true, 1001: true, 0: false, 1002: false} {
		if Allowed(uid, set) != want {
			t.Fatalf("Allowed(%d) != %v", uid, want)
		}
	}
	if Allowed(1000, nil) {
		t.Fatal("nil set must deny")
	}
}
