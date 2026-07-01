// SPDX-License-Identifier: Apache-2.0
package attach

import (
	"path/filepath"
	"testing"
)

func TestPinPathDerivation(t *testing.T) {
	a := &Attacher{cfg: testCfg()}
	if got := a.netnsPath("cidX"); got != filepath.Join("/run/xnetd", "netns", "cidX") {
		t.Fatalf("path = %q", got)
	}
}

func TestUnpinNetnsTempStateDir(t *testing.T) {
	cfg := testCfg()
	cfg.Runtime.StateDir = t.TempDir()
	a := &Attacher{cfg: cfg}
	// No pin present: UnpinNetns attempts unmount(ignored)+Remove(missing) -> error surfaced.
	if err := a.UnpinNetns("nope"); err == nil {
		t.Fatal("unpin of absent pin should return the remove error")
	}
}
