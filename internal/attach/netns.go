// SPDX-License-Identifier: Apache-2.0
package attach

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// PinNetns bind-mounts the received netns fd to a stable path so it survives
// the hook process exiting and the daemon closing the fd.
func (a *Attacher) PinNetns(fd int, cid string) (string, error) {
	dst := a.netnsPath(cid)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", err
	}
	f.Close()
	_ = unix.Unmount(dst, unix.MNT_DETACH) // clear any stale bind
	src := fmt.Sprintf("/proc/self/fd/%d", fd)
	if err := unix.Mount(src, dst, "none", unix.MS_BIND, ""); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("attach: bind netns %s: %w", dst, err)
	}
	return dst, nil
}

// UnpinNetns unmounts and removes the pinned netns for cid.
func (a *Attacher) UnpinNetns(cid string) error {
	dst := a.netnsPath(cid)
	_ = unix.Unmount(dst, unix.MNT_DETACH)
	return os.Remove(dst)
}
