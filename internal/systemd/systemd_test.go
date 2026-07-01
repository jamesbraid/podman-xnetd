// SPDX-License-Identifier: Apache-2.0
package systemd

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestListenerOrNilWithoutActivation(t *testing.T) {
	os.Unsetenv("LISTEN_PID")
	os.Unsetenv("LISTEN_FDS")
	os.Unsetenv("LISTEN_FDNAMES")
	ln, err := ListenerOrNil()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ln != nil {
		t.Fatalf("want nil listener, got %T", ln)
	}
}

func TestWatchdogLoopReturnsWhenDisabled(t *testing.T) {
	os.Unsetenv("WATCHDOG_USEC")
	os.Unsetenv("WATCHDOG_PID")
	done := make(chan struct{})
	go func() { WatchdogLoop(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WatchdogLoop did not return when disabled")
	}
}
