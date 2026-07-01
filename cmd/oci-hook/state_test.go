// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, bundle, root string) {
	t.Helper()
	os.MkdirAll(bundle, 0o755)
	os.WriteFile(filepath.Join(bundle, "config.json"), []byte(`{"root":{"path":"`+root+`"}}`), 0o644)
}

func TestRootfsFromBundleRelative(t *testing.T) {
	b := t.TempDir()
	writeConfig(t, b, "rootfs")
	got, err := rootfsFromBundle(b)
	if err != nil || got != filepath.Join(b, "rootfs") {
		t.Fatalf("got %q err %v", got, err)
	}
}
func TestRootfsFromBundleAbsolute(t *testing.T) {
	b := t.TempDir()
	writeConfig(t, b, "/var/lib/x/rootfs")
	got, _ := rootfsFromBundle(b)
	if got != "/var/lib/x/rootfs" {
		t.Fatalf("got %q", got)
	}
}
func TestParseState(t *testing.T) {
	st, err := parseState(strings.NewReader(`{"id":"abc","pid":4321,"bundle":"/b","annotations":{"org.octanix.rootful_networks":"lan"}}`))
	if err != nil || st.ID != "abc" || st.Pid != 4321 || st.Bundle != "/b" {
		t.Fatalf("st=%+v err=%v", st, err)
	}
}
func TestBuildAttachRequest(t *testing.T) {
	b := t.TempDir()
	writeConfig(t, b, "rootfs")
	req, err := buildAttachRequest(ociState{ID: "cid1", Pid: 999, Bundle: b,
		Annotations: map[string]string{"org.octanix.rootful_networks": "lan,dmz", "org.octanix.static_ip.lan": "10.0.0.2"}})
	if err != nil || req.Action != "attach" || req.ContainerID != "cid1" || req.ContainerName != "cid1" ||
		req.Pid != 999 || req.RootfsPath != filepath.Join(b, "rootfs") ||
		!reflect.DeepEqual(req.Networks, []string{"lan", "dmz"}) ||
		!reflect.DeepEqual(req.StaticIPs, map[string][]string{"lan": {"10.0.0.2"}}) {
		t.Fatalf("req=%+v err=%v", req, err)
	}
}
