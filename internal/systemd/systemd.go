// SPDX-License-Identifier: Apache-2.0

// Package systemd wraps the systemd integration points (socket activation,
// readiness notification, watchdog) with pure-Go stdlib-friendly helpers.
package systemd

import (
	"context"
	"net"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/coreos/go-systemd/v22/daemon"
)

func ListenerOrNil() (net.Listener, error) {
	ls, err := activation.Listeners()
	if err != nil {
		return nil, err
	}
	if len(ls) == 0 {
		return nil, nil
	}
	return ls[0], nil
}

func NotifyReady() { _, _ = daemon.SdNotify(false, daemon.SdNotifyReady) }

func WatchdogLoop(ctx context.Context) {
	interval, err := daemon.SdWatchdogEnabled(false)
	if err != nil || interval == 0 {
		return
	}
	tick := interval / 2
	if tick <= 0 {
		tick = interval
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = daemon.SdNotify(false, daemon.SdNotifyWatchdog)
		}
	}
}
