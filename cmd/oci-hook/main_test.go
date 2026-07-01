// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"testing"
)

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
