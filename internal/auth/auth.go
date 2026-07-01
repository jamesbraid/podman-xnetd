// SPDX-License-Identifier: Apache-2.0

// Package auth is the daemon's SO_PEERCRED uid-allowlist gate.
package auth

import (
	"net"

	"golang.org/x/sys/unix"
)

// PeerUID returns the connecting peer's uid via SO_PEERCRED (kernel-supplied,
// unforgeable).
func PeerUID(c *net.UnixConn) (int, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return -1, err
	}
	var ucred *unix.Ucred
	var operr error
	if err := raw.Control(func(fd uintptr) {
		ucred, operr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return -1, err
	}
	if operr != nil {
		return -1, operr
	}
	return int(ucred.Uid), nil
}

// Allowed reports whether uid is in the allowlist (nil denies all).
func Allowed(uid int, allowed map[int]struct{}) bool {
	_, ok := allowed[uid]
	return ok
}
