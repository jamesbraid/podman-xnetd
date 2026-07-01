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
	if fd >= 0 {
		return fmt.Errorf("proto: fd passing not implemented")
	}
	_, err = c.Write(frame(body))
	return err
}

func ReadRequest(c *net.UnixConn) (Request, int, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return Request{}, -1, err
	}
	body := make([]byte, binary.BigEndian.Uint32(hdr[:]))
	if _, err := io.ReadFull(c, body); err != nil {
		return Request{}, -1, err
	}
	var r Request
	if err := json.Unmarshal(body, &r); err != nil {
		return Request{}, -1, err
	}
	return r, -1, nil
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
