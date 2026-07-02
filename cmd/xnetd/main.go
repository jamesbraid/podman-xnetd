// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/jamesbraid/podman-xnetd/internal/attach"
	"github.com/jamesbraid/podman-xnetd/internal/auth"
	"github.com/jamesbraid/podman-xnetd/internal/config"
	"github.com/jamesbraid/podman-xnetd/internal/neighbor"
	"github.com/jamesbraid/podman-xnetd/internal/proto"
	"github.com/jamesbraid/podman-xnetd/internal/reconcile"
	"github.com/jamesbraid/podman-xnetd/internal/systemd"
	"go.podman.io/common/libnetwork/types"
	"golang.org/x/sys/unix"
)

var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("xnetd", flag.ContinueOnError)
	fs.SetOutput(stdout)
	cfgPath := fs.String("config", "/etc/xnetd/config.toml", "config path")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	return serve(*cfgPath)
}

type server struct {
	att      *attach.Attacher
	locks    *cidLocks
	stateDir string
	mu       sync.RWMutex
	allowed  map[int]struct{}
}

func (s *server) setAllowed(m map[int]struct{}) { s.mu.Lock(); s.allowed = m; s.mu.Unlock() }
func (s *server) allowedSet() map[int]struct{}  { s.mu.RLock(); defer s.mu.RUnlock(); return s.allowed }

func (s *server) reload(cfgPath string) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("xnetd: reload load: %v", err)
		return
	}
	uids, err := cfg.ResolveAllowedUIDs()
	if err != nil {
		log.Printf("xnetd: reload uids: %v", err)
		return
	}
	s.setAllowed(uids)
	log.Printf("xnetd: reloaded allowlist (%d uids)", len(uids))
}

func serve(cfgPath string) int {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("xnetd: load config: %v", err)
		return 1
	}
	uids, err := cfg.ResolveAllowedUIDs()
	if err != nil {
		log.Printf("xnetd: resolve uids: %v", err)
		return 1
	}
	att, err := attach.New(cfg)
	if err != nil {
		log.Printf("xnetd: attacher: %v", err)
		return 1
	}
	if err := reconcile.Reconcile(att, cfg.Runtime.StateDir); err != nil {
		log.Printf("xnetd: reconcile: %v", err)
	}
	ln, err := systemd.ListenerOrNil()
	if err != nil {
		log.Printf("xnetd: activation: %v", err)
		return 1
	}
	if ln == nil {
		ln, err = net.Listen("unix", cfg.Runtime.Socket)
		if err != nil {
			log.Printf("xnetd: listen %s: %v", cfg.Runtime.Socket, err)
			return 1
		}
		if err := os.Chmod(cfg.Runtime.Socket, 0o666); err != nil {
			log.Printf("xnetd: chmod: %v", err)
			return 1
		}
	}
	srv := &server{att: att, locks: newCidLocks(), stateDir: cfg.Runtime.StateDir}
	srv.setAllowed(uids)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go systemd.WatchdogLoop(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGHUP:
				srv.reload(cfgPath)
			case syscall.SIGTERM, syscall.SIGINT:
				signal.Stop(sigCh)
				cancel()
				_ = ln.Close()
				return
			}
		}
	}()
	systemd.NotifyReady()
	log.Printf("xnetd: listening")
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return 0
			default:
				log.Printf("xnetd: accept: %v", err)
				return 1
			}
		}
		uc, ok := conn.(*net.UnixConn)
		if !ok {
			_ = conn.Close()
			continue
		}
		go srv.handleConn(uc)
	}
}

func (s *server) handleConn(conn *net.UnixConn) {
	defer conn.Close()
	uid, err := auth.PeerUID(conn)
	if err != nil {
		log.Printf("xnetd: peer uid: %v", err)
		return
	}
	if !auth.Allowed(uid, s.allowedSet()) {
		log.Printf("xnetd: rejected uid %d", uid)
		_ = proto.WriteResponse(conn, proto.Response{OK: false, Error: "uid not allowed"})
		return
	}
	req, fd, err := proto.ReadRequest(conn)
	if fd >= 0 {
		defer unix.Close(fd)
	}
	if err != nil {
		log.Printf("xnetd: read request: %v", err)
		return
	}
	if err := proto.WriteResponse(conn, s.handleRequest(req, fd)); err != nil {
		log.Printf("xnetd: write response: %v", err)
	}
}

func (s *server) handleRequest(req proto.Request, fd int) proto.Response {
	lock := s.locks.get(req.ContainerID)
	lock.Lock()
	defer lock.Unlock()
	switch req.Action {
	case "attach":
		return s.doAttach(req, fd)
	case "detach":
		return s.doDetach(req)
	default:
		return proto.Response{OK: false, Error: "unknown action: " + req.Action}
	}
}

func (s *server) doAttach(req proto.Request, fd int) proto.Response {
	netnsPath, err := s.att.PinNetns(fd, req.ContainerID)
	if err != nil {
		return proto.Response{OK: false, Error: "pin netns: " + err.Error()}
	}
	status, err := s.att.Attach(req)
	if err != nil {
		_ = s.att.UnpinNetns(req.ContainerID)
		return proto.Response{OK: false, Error: "attach: " + err.Error()}
	}
	if err := reconcile.WriteAttachCfg(s.stateDir, req.ContainerID, reconcile.AttachCfg{
		Networks:  req.Networks,
		StaticIPs: req.StaticIPs,
	}); err != nil {
		log.Printf("xnetd: persist cfg %s: %v", req.ContainerID, err)
	}
	if err := attach.WriteResolvConf(req, status); err != nil {
		log.Printf("xnetd: resolv.conf %s: %v", req.ContainerID, err)
	}
	announce(netnsPath, status)
	return proto.Response{OK: true}
}

func (s *server) doDetach(req proto.Request) proto.Response {
	// If the hook didn't supply network info (poststop only sends container_id),
	// load the saved attach config so netavark can tear down correctly and the
	// IPAM lease is freed.
	if len(req.Networks) == 0 {
		if cfg, ok, err := reconcile.ReadAttachCfg(s.stateDir, req.ContainerID); err != nil {
			log.Printf("xnetd: read cfg %s: %v (detaching without networks)", req.ContainerID, err)
		} else if ok {
			req.Networks = cfg.Networks
			req.StaticIPs = cfg.StaticIPs
		}
	}
	if err := s.att.Detach(req); err != nil {
		return proto.Response{OK: false, Error: "detach: " + err.Error()}
	}
	if err := s.att.UnpinNetns(req.ContainerID); err != nil {
		return proto.Response{OK: false, Error: "unpin: " + err.Error()}
	}
	reconcile.RemoveAttachCfg(s.stateDir, req.ContainerID)
	return proto.Response{OK: true}
}

func announce(netnsPath string, status map[string]types.StatusBlock) {
	for _, sb := range status {
		for ifaceName, iface := range sb.Interfaces {
			var addrs []net.IP
			for _, sn := range iface.Subnets {
				if sn.IPNet.IP != nil {
					addrs = append(addrs, sn.IPNet.IP)
				}
			}
			if len(addrs) == 0 {
				continue
			}
			if err := neighbor.Announce(netnsPath, ifaceName, addrs, net.HardwareAddr(iface.MacAddress)); err != nil {
				log.Printf("xnetd: announce %s: %v", ifaceName, err)
			}
		}
	}
}
