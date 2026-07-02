// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jamesbraid/podman-xnetd/internal/proto"
)

type ociState struct {
	ID          string            `json:"id"`
	Pid         int               `json:"pid"`
	Bundle      string            `json:"bundle"`
	Annotations map[string]string `json:"annotations"`
}

func parseState(r io.Reader) (ociState, error) {
	var st ociState
	if err := json.NewDecoder(r).Decode(&st); err != nil {
		return ociState{}, err
	}
	return st, nil
}

func rootfsFromBundle(bundle string) (string, error) {
	data, err := os.ReadFile(filepath.Join(bundle, "config.json"))
	if err != nil {
		return "", err
	}
	var spec struct {
		Root struct {
			Path string `json:"path"`
		} `json:"root"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return "", err
	}
	if spec.Root.Path == "" {
		return "", fmt.Errorf("rootfsFromBundle: empty root.path in %s", bundle)
	}
	if !filepath.IsAbs(spec.Root.Path) {
		return filepath.Join(bundle, spec.Root.Path), nil
	}
	return spec.Root.Path, nil
}

func buildAttachRequest(st ociState) (proto.Request, error) {
	networks, staticIPs, name := parseAnnotations(st.Annotations)
	if name == "" {
		name = st.ID
	}
	rootfs, err := rootfsFromBundle(st.Bundle)
	if err != nil {
		return proto.Request{}, err
	}
	return proto.Request{Action: "attach", ContainerID: st.ID, ContainerName: name,
		Networks: networks, StaticIPs: staticIPs, Pid: st.Pid, RootfsPath: rootfs}, nil
}
