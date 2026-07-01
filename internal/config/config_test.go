// SPDX-License-Identifier: Apache-2.0
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "xnetd.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValid(t *testing.T) {
	p := writeTOML(t, `
allowed_users = ["alice", "bob"]
[paths]
netavark = "/usr/lib/podman/netavark"
aardvark = "/usr/lib/podman/aardvark-dns"
[libnetwork]
network_config_dir = "/etc/containers/networks"
network_run_dir = "/run/containers/networks"
[runtime]
state_dir = "/run/xnetd"
socket = "/run/xnetd/sock"
`)
	got, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := &Config{
		AllowedUsers: []string{"alice", "bob"},
		Paths:        PathsConfig{Netavark: "/usr/lib/podman/netavark", Aardvark: "/usr/lib/podman/aardvark-dns"},
		Libnetwork:   LibnetworkConfig{NetworkConfigDir: "/etc/containers/networks", NetworkRunDir: "/run/containers/networks"},
		Runtime:      RuntimeConfig{StateDir: "/run/xnetd", Socket: "/run/xnetd/sock"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mismatch\n got %+v\n want %+v", got, want)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	p := writeTOML(t, "allowed_users = [\"a\"]\nbogus_key = \"nope\"\n")
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "bogus_key") {
		t.Fatalf("want strict error naming bogus_key, got %v", err)
	}
}
