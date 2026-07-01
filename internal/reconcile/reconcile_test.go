// SPDX-License-Identifier: Apache-2.0
package reconcile

import (
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
