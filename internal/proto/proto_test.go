// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 James Braid

package proto

import (
	"net"
	"os"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"
)

func socketPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	mk := func(fd int) *net.UnixConn {
		f := os.NewFile(uintptr(fd), "sock")
		c, err := net.FileConn(f)
		if err != nil {
			t.Fatalf("FileConn: %v", err)
		}
		f.Close()
		return c.(*net.UnixConn)
	}
	return mk(fds[0]), mk(fds[1])
}

func TestRequestRoundTripNoFD(t *testing.T) {
	a, b := socketPair(t)
	defer a.Close()
	defer b.Close()
	want := Request{Action: "attach", ContainerID: "abc", ContainerName: "web",
		Networks: []string{"net0", "net1"}, StaticIPs: map[string][]string{"net0": {"10.0.0.5"}},
		Pid: 4242, RootfsPath: "/run/rootfs"}
	errc := make(chan error, 1)
	go func() { errc <- WriteRequest(a, want, -1) }()
	got, fd, err := ReadRequest(b)
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if werr := <-errc; werr != nil {
		t.Fatalf("WriteRequest: %v", werr)
	}
	if fd != -1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("mismatch fd=%d got=%+v", fd, got)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	a, b := socketPair(t)
	defer a.Close()
	defer b.Close()
	want := Response{OK: false, Error: "boom", Log: "l1\nl2"}
	errc := make(chan error, 1)
	go func() { errc <- WriteResponse(a, want) }()
	got, err := ReadResponse(b)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if werr := <-errc; werr != nil {
		t.Fatalf("WriteResponse: %v", werr)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mismatch got=%+v", got)
	}
}
