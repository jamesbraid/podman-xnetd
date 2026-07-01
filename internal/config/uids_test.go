// SPDX-License-Identifier: Apache-2.0
package config

import (
	"os/user"
	"strconv"
	"testing"
)

func TestResolveAllowedUIDsCurrentUser(t *testing.T) {
	me, _ := user.Current()
	wantUID, _ := strconv.Atoi(me.Uid)
	set, err := (&Config{AllowedUsers: []string{me.Username}}).ResolveAllowedUIDs()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, ok := set[wantUID]; !ok || len(set) != 1 {
		t.Fatalf("set=%v want {%d}", set, wantUID)
	}
}

func TestResolveAllowedUIDsUnknown(t *testing.T) {
	if _, err := (&Config{AllowedUsers: []string{"no-such-user-xnetd"}}).ResolveAllowedUIDs(); err == nil {
		t.Fatal("expected error for unknown user")
	}
}
