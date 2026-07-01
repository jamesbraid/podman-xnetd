// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/jamesbraid/xnetd/internal/proto"
	"golang.org/x/sys/unix"
)

func main() { os.Exit(run(os.Args, os.Stdin)) }

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
	stage := resolveStage(args)
	st, err := parseState(stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oci-hook: parse state: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "oci-hook: unknown stage %q\n", stage)
		return 1
	}
}

func doCreateRuntime(st ociState, socket string) int {
	if st.Pid <= 0 {
		fmt.Fprintln(os.Stderr, "oci-hook: createRuntime needs pid>0")
		return 1
	}
	nsPath := fmt.Sprintf("/proc/%d/ns/net", st.Pid)
	fd, err := unix.Open(nsPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oci-hook: open %s: %v\n", nsPath, err)
		return 1
	}
	defer unix.Close(fd)
	req, err := buildAttachRequest(st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oci-hook: build request: %v\n", err)
		return 1
	}
	resp, err := roundTrip(socket, req, fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oci-hook: request: %v\n", err)
		return 1
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "oci-hook: attach failed: %s\n", resp.Error)
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
