// SPDX-License-Identifier: Apache-2.0

// Package reconcile reaps netns pins left by containers that exited while the
// daemon was down.
package reconcile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/jamesbraid/xnetd/internal/attach"
	"github.com/jamesbraid/xnetd/internal/proto"
)

// netnsIsDead reports whether the pin at path is no longer a live bind mount:
// a live pin crosses a filesystem boundary (different st_dev than its parent);
// a matching device (or a missing path) means it is dead.
func netnsIsDead(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return true
	}
	pfi, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return true
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	pst, ok2 := pfi.Sys().(*syscall.Stat_t)
	if !ok || !ok2 {
		return true
	}
	return st.Dev == pst.Dev
}

func deadNetns(stateDir string) ([]string, error) {
	nd := filepath.Join(stateDir, "netns")
	entries, err := os.ReadDir(nd)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dead []string
	for _, e := range entries {
		if netnsIsDead(filepath.Join(nd, e.Name())) {
			dead = append(dead, e.Name())
		}
	}
	return dead, nil
}

// Reconcile detaches + unpins every container whose netns pin is dead.
func Reconcile(a *attach.Attacher, stateDir string) error {
	dead, err := deadNetns(stateDir)
	if err != nil {
		return err
	}
	var errs []error
	for _, cid := range dead {
		if derr := a.Detach(proto.Request{Action: "detach", ContainerID: cid}); derr != nil {
			errs = append(errs, fmt.Errorf("detach %s: %w", cid, derr))
		}
		if uerr := a.UnpinNetns(cid); uerr != nil {
			errs = append(errs, fmt.Errorf("unpin %s: %w", cid, uerr))
		}
	}
	return errors.Join(errs...)
}
