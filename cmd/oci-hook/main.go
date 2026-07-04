// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/jamesbraid/podman-xnetd/internal/proto"
	"golang.org/x/sys/unix"
)

var version = "dev"

func main() { os.Exit(run(os.Args, os.Stdin)) }

// logErr reports a hook failure to stderr AND the journal. A createRuntime hook
// that exits non-zero aborts the container start, but crun discards the hook's
// stderr — the journal (journalctl -t xnet-oci-hook) is the only place an
// operator can see why. Journal send is best-effort: if journald is unreachable
// the stderr line still stands.
func logErr(format string, a ...any) {
	msg := "oci-hook: " + fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, msg)
	_ = journal.Send(msg, journal.PriErr, map[string]string{"SYSLOG_IDENTIFIER": "xnet-oci-hook"})
}

func resolveStage(args []string) string {
	if len(args) > 1 && args[1] != "" {
		return args[1]
	}
	return os.Getenv("XNET_HOOK_STAGE")
}

func hookSocket() string {
	if s := os.Getenv("XNET_HOOK_SOCKET"); s != "" {
		return s
	}
	return "/run/xnetd/sock"
}

func run(args []string, stdin io.Reader) int {
	// Check for --version early
	for _, arg := range args {
		if arg == "--version" {
			fmt.Println(version)
			return 0
		}
	}
	stage := resolveStage(args)
	st, err := parseState(stdin)
	if err != nil {
		logErr("parse state: %v", err)
		return 1
	}
	socket := hookSocket()
	switch stage {
	case "createRuntime":
		return doCreateRuntime(st, socket)
	case "poststop":
		doPoststop(st, socket)
		return 0
	default:
		logErr("unknown stage %q", stage)
		return 1
	}
}

func doCreateRuntime(st ociState, socket string) int {
	if st.Pid <= 0 {
		logErr("cid=%s createRuntime needs pid>0", st.ID)
		return 1
	}
	nsPath := fmt.Sprintf("/proc/%d/ns/net", st.Pid)
	fd, err := unix.Open(nsPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		logErr("cid=%s open %s: %v", st.ID, nsPath, err)
		return 1
	}
	defer unix.Close(fd)
	req, err := buildAttachRequest(st)
	if err != nil {
		logErr("cid=%s build request: %v", st.ID, err)
		return 1
	}
	resp, err := roundTrip(socket, req, fd)
	if err != nil {
		logErr("cid=%s request: %v", st.ID, err)
		return 1
	}
	if !resp.OK {
		logErr("cid=%s attach failed: %s", st.ID, resp.Error)
		return 1
	}
	return 0
}

func doPoststop(st ociState, socket string) {
	_, _ = roundTrip(socket, proto.Request{Action: "detach", ContainerID: st.ID}, -1)
}

func roundTrip(socket string, req proto.Request, fd int) (proto.Response, error) {
	c, err := net.Dial("unix", socket)
	if err != nil {
		return proto.Response{}, err
	}
	defer c.Close()
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return proto.Response{}, fmt.Errorf("oci-hook: not a unix conn")
	}
	if err := proto.WriteRequest(uc, req, fd); err != nil {
		return proto.Response{}, err
	}
	return proto.ReadResponse(uc)
}
