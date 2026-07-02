// SPDX-License-Identifier: Apache-2.0

// Package reconcile reaps netns pins left by containers that exited while the
// daemon was down.
package reconcile

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/jamesbraid/podman-xnetd/internal/attach"
	"github.com/jamesbraid/podman-xnetd/internal/proto"
)

// AttachCfg is the per-container state persisted at attach time so reconcile
// can supply the correct network names to libnetwork Teardown and release the
// IPAM lease for containers that died while the daemon was down.
type AttachCfg struct {
	Networks  []string            `json:"networks"`
	StaticIPs map[string][]string `json:"static_ips,omitempty"`
}

func cfgPath(stateDir, cid string) string {
	return filepath.Join(stateDir, "cfg", cid+".json")
}

// WriteAttachCfg persists the network config for cid to disk.
func WriteAttachCfg(stateDir, cid string, cfg AttachCfg) error {
	dir := filepath.Join(stateDir, "cfg")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("reconcile: mkdir cfg: %w", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("reconcile: marshal cfg: %w", err)
	}
	if err := os.WriteFile(cfgPath(stateDir, cid), data, 0o600); err != nil {
		return fmt.Errorf("reconcile: write cfg: %w", err)
	}
	return nil
}

// RemoveAttachCfg removes the persisted config for cid; ignores missing files.
func RemoveAttachCfg(stateDir, cid string) {
	_ = os.Remove(cfgPath(stateDir, cid))
}

// ReadAttachCfg reads the persisted config for cid. Returns (cfg, true) on
// success, ({}, false) if the file is missing, and an error on other failures.
func ReadAttachCfg(stateDir, cid string) (AttachCfg, bool, error) {
	data, err := os.ReadFile(cfgPath(stateDir, cid))
	if err != nil {
		if os.IsNotExist(err) {
			return AttachCfg{}, false, nil
		}
		return AttachCfg{}, false, err
	}
	var cfg AttachCfg
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AttachCfg{}, false, err
	}
	return cfg, true, nil
}

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
// It reads the persisted AttachCfg for each dead container so that
// libnetwork Teardown can release the IPAM lease; if the file is missing it
// logs and still unpins (best-effort).
func Reconcile(a *attach.Attacher, stateDir string) error {
	dead, err := deadNetns(stateDir)
	if err != nil {
		return err
	}
	var errs []error
	for _, cid := range dead {
		req := proto.Request{Action: "detach", ContainerID: cid}
		if cfg, ok, rerr := ReadAttachCfg(stateDir, cid); rerr != nil {
			log.Printf("reconcile: read cfg %s: %v (detaching without networks)", cid, rerr)
		} else if ok {
			req.Networks = cfg.Networks
			req.StaticIPs = cfg.StaticIPs
		} else {
			log.Printf("reconcile: no cfg for %s; detaching without networks (IPAM may leak)", cid)
		}
		if derr := a.Detach(req); derr != nil {
			errs = append(errs, fmt.Errorf("detach %s: %w", cid, derr))
		}
		if uerr := a.UnpinNetns(cid); uerr != nil {
			errs = append(errs, fmt.Errorf("unpin %s: %w", cid, uerr))
		}
		RemoveAttachCfg(stateDir, cid)
	}
	return errors.Join(errs...)
}
