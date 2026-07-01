// SPDX-License-Identifier: Apache-2.0
package main

import "sync"

// cidLocks serializes attach/detach per container while allowing distinct
// containers to proceed in parallel.
type cidLocks struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

func newCidLocks() *cidLocks { return &cidLocks{m: map[string]*sync.Mutex{}} }

func (c *cidLocks) get(cid string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.m[cid]
	if !ok {
		l = &sync.Mutex{}
		c.m[cid] = l
	}
	return l
}
