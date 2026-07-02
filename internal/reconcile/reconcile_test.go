// SPDX-License-Identifier: Apache-2.0
package reconcile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNetnsIsDeadForNonMountpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "netns", "abc")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, nil, 0o644)
	if !netnsIsDead(path) {
		t.Fatal("non-mountpoint pin must be dead")
	}
	if !netnsIsDead(filepath.Join(dir, "netns", "gone")) {
		t.Fatal("missing path must be dead")
	}
}

func TestDeadNetnsLists(t *testing.T) {
	dir := t.TempDir()
	nd := filepath.Join(dir, "netns")
	os.MkdirAll(nd, 0o755)
	os.WriteFile(filepath.Join(nd, "c1"), nil, 0o644)
	os.WriteFile(filepath.Join(nd, "c2"), nil, 0o644)
	got, err := deadNetns(dir)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestDeadNetnsMissingDir(t *testing.T) {
	got, err := deadNetns(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestWriteAndReadAttachCfg(t *testing.T) {
	dir := t.TempDir()
	want := AttachCfg{
		Networks:  []string{"net0", "net1"},
		StaticIPs: map[string][]string{"net0": {"10.0.0.5"}},
	}
	if err := WriteAttachCfg(dir, "cid1", want); err != nil {
		t.Fatalf("WriteAttachCfg: %v", err)
	}
	got, ok, err := ReadAttachCfg(dir, "cid1")
	if err != nil || !ok {
		t.Fatalf("readAttachCfg: ok=%v err=%v", ok, err)
	}
	if len(got.Networks) != 2 || got.Networks[0] != "net0" || got.Networks[1] != "net1" {
		t.Fatalf("networks mismatch: %v", got.Networks)
	}
	if len(got.StaticIPs["net0"]) != 1 || got.StaticIPs["net0"][0] != "10.0.0.5" {
		t.Fatalf("static_ips mismatch: %v", got.StaticIPs)
	}
}

func TestReadAttachCfgMissing(t *testing.T) {
	dir := t.TempDir()
	_, ok, err := ReadAttachCfg(dir, "no-such-cid")
	if err != nil || ok {
		t.Fatalf("expected missing: ok=%v err=%v", ok, err)
	}
}

func TestReadAttachCfgCorrupt(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "cfg"), 0o700)
	os.WriteFile(filepath.Join(dir, "cfg", "bad.json"), []byte("not json"), 0o600)
	_, ok, err := ReadAttachCfg(dir, "bad")
	if err == nil || ok {
		t.Fatalf("expected error for corrupt file: ok=%v err=%v", ok, err)
	}
}

func TestRemoveAttachCfg(t *testing.T) {
	dir := t.TempDir()
	if err := WriteAttachCfg(dir, "c1", AttachCfg{Networks: []string{"net0"}}); err != nil {
		t.Fatalf("WriteAttachCfg: %v", err)
	}
	RemoveAttachCfg(dir, "c1")
	if _, err := os.Stat(cfgPath(dir, "c1")); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}
	// removing non-existent is a no-op
	RemoveAttachCfg(dir, "no-such")
}

func TestWriteAttachCfgRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := AttachCfg{Networks: []string{"mynet"}}
	if err := WriteAttachCfg(dir, "abc", cfg); err != nil {
		t.Fatalf("WriteAttachCfg: %v", err)
	}
	data, err := os.ReadFile(cfgPath(dir, "abc"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got AttachCfg
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Networks) != 1 || got.Networks[0] != "mynet" {
		t.Fatalf("unexpected: %v", got)
	}
}
