// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 James Braid

// Package proto is the xnetd wire protocol: 4-byte big-endian length prefix
// + JSON body; attach passes one netns fd via SCM_RIGHTS.
package proto

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"golang.org/x/sys/unix"
)

type Request struct {
	Action        string              `json:"action"`
	ContainerID   string              `json:"container_id"`
	ContainerName string              `json:"container_name"`
	Networks      []string            `json:"networks"`
	StaticIPs     map[string][]string `json:"static_ips,omitempty"`
	Pid           int                 `json:"pid,omitempty"`
	RootfsPath    string              `json:"rootfs_path,omitempty"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Log   string `json:"log,omitempty"`
}

func frame(body []byte) []byte {
	buf := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(body)))
	copy(buf[4:], body)
	return buf
}

func WriteRequest(c *net.UnixConn, r Request, fd int) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	buf := frame(body)
	var oob []byte
	if fd >= 0 {
		oob = unix.UnixRights(fd)
	}
	n, oobn, err := c.WriteMsgUnix(buf, oob, nil)
	if err != nil {
		return err
	}
	if n != len(buf) || oobn != len(oob) {
		return fmt.Errorf("proto: short write %d/%d, oob %d/%d", n, len(buf), oobn, len(oob))
	}
	return nil
}

func ReadRequest(c *net.UnixConn) (Request, int, error) {
	var hdr [4]byte
	oob := make([]byte, unix.CmsgSpace(4))
	n, oobn, _, _, err := c.ReadMsgUnix(hdr[:], oob)
	if err != nil {
		return Request{}, -1, err
	}
	if n != len(hdr) {
		return Request{}, -1, fmt.Errorf("proto: short header %d/%d", n, len(hdr))
	}
	fd := -1
	if oobn > 0 {
		scms, perr := unix.ParseSocketControlMessage(oob[:oobn])
		if perr != nil {
			return Request{}, -1, perr
		}
		for _, scm := range scms {
			if fds, ferr := unix.ParseUnixRights(&scm); ferr == nil && len(fds) > 0 {
				fd = fds[0]
			}
		}
	}
	bodyLen := binary.BigEndian.Uint32(hdr[:])
	if bodyLen > 1<<20 {
		return Request{}, fd, fmt.Errorf("proto: request body too large (%d bytes)", bodyLen)
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(c, body); err != nil {
		return Request{}, fd, err
	}
	var r Request
	if err := json.Unmarshal(body, &r); err != nil {
		return Request{}, fd, err
	}
	return r, fd, nil
}

func WriteResponse(c *net.UnixConn, r Response) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = c.Write(frame(body))
	return err
}

func ReadResponse(c *net.UnixConn) (Response, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return Response{}, err
	}
	body := make([]byte, binary.BigEndian.Uint32(hdr[:]))
	if _, err := io.ReadFull(c, body); err != nil {
		return Response{}, err
	}
	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		return Response{}, err
	}
	return r, nil
}
