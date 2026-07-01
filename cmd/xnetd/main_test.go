// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	version = "1.2.3"
	var buf bytes.Buffer
	if code := run([]string{"--version"}, &buf); code != 0 || strings.TrimSpace(buf.String()) != "1.2.3" {
		t.Fatalf("code=%d out=%q", code, buf.String())
	}
}
func TestRunBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if run([]string{"--nope"}, &buf) != 2 {
		t.Fatal("bad flag should return 2")
	}
}
