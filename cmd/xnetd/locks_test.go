// SPDX-License-Identifier: Apache-2.0
package main

import (
	"sync"
	"testing"
)

func TestCidLocksStableIdentity(t *testing.T) {
	l := newCidLocks()
	if l.get("c1") != l.get("c1") || l.get("c2") == l.get("c1") {
		t.Fatal("lock identity wrong")
	}
}
func TestCidLocksConcurrent(t *testing.T) {
	l := newCidLocks()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); m := l.get("same"); m.Lock(); m.Unlock() }()
	}
	wg.Wait()
}
