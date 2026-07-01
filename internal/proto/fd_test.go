// SPDX-License-Identifier: Apache-2.0
package proto

import (
	"os"
	"testing"
)

func TestRequestPassesFD(t *testing.T) {
	a, b := socketPair(t)
	defer a.Close()
	defer b.Close()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pr.Close()
	defer pw.Close()
	pw.WriteString("ping")
	req := Request{Action: "attach", ContainerID: "c1", Networks: []string{"net0"}, Pid: 99}
	errc := make(chan error, 1)
	go func() { errc <- WriteRequest(a, req, int(pr.Fd())) }()
	got, fd, err := ReadRequest(b)
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if werr := <-errc; werr != nil {
		t.Fatalf("WriteRequest: %v", werr)
	}
	if got.ContainerID != "c1" || fd < 0 {
		t.Fatalf("payload/fd mismatch: %+v fd=%d", got, fd)
	}
	recv := os.NewFile(uintptr(fd), "recv")
	defer recv.Close()
	buf := make([]byte, 4)
	recv.Read(buf)
	if string(buf) != "ping" {
		t.Fatalf("received fd content = %q", buf)
	}
}
