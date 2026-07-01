// SPDX-License-Identifier: Apache-2.0

// Package neighbor sends gratuitous ARP / unsolicited NDP NA inside a netns so
// peers relearn a container's MAC when it is recreated with the same IP.
package neighbor

import (
	"encoding/binary"
	"net"
)

func buildGratuitousARP(ip net.IP, mac net.HardwareAddr) []byte {
	v4 := ip.To4()
	p := make([]byte, 28)
	binary.BigEndian.PutUint16(p[0:2], 1)      // htype Ethernet
	binary.BigEndian.PutUint16(p[2:4], 0x0800) // ptype IPv4
	p[4] = 6                                    // hlen
	p[5] = 4                                    // plen
	binary.BigEndian.PutUint16(p[6:8], 1)       // op request
	copy(p[8:14], mac)
	copy(p[14:18], v4)
	copy(p[18:24], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	copy(p[24:28], v4)
	return p
}
