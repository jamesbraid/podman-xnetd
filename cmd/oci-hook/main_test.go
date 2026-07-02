// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"testing"
)

func TestOciHookVersion(t *testing.T) {
	version = "1.2.3"
	var buf bytes.Buffer
	if code := run([]string{"oci-hook", "--version"}, &buf); code != 0 {
		t.Fatalf("--version should return 0, got %d", code)
	}
}

func TestResolveStagePrefersArg(t *testing.T) {
	t.Setenv("XNET_HOOK_STAGE", "poststop")
	if resolveStage([]string{"oci-hook", "createRuntime"}) != "createRuntime" {
		t.Fatal("argv[1] must win")
	}
}
func TestResolveStageEnvFallback(t *testing.T) {
	t.Setenv("XNET_HOOK_STAGE", "poststop")
	if resolveStage([]string{"oci-hook"}) != "poststop" {
		t.Fatal("env fallback")
	}
}
func TestHookSocketDefault(t *testing.T) {
	os.Unsetenv("XNET_HOOK_SOCKET")
	if hookSocket() != "/run/xnetd/sock" {
		t.Fatal("default socket")
	}
	t.Setenv("XNET_HOOK_SOCKET", "/tmp/x.sock")
	if hookSocket() != "/tmp/x.sock" {
		t.Fatal("env override")
	}
}
