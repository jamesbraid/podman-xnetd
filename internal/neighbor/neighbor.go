// SPDX-License-Identifier: Apache-2.0

// Package neighbor sends gratuitous ARP / unsolicited NDP NA inside a netns so
// peers relearn a container's MAC when it is recreated with the same IP.
package neighbor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
)

const (
	icmpTypeNA      = 136
	naFlagOverride  = 0x20 // R0 S0 O1
	optTargetLLAddr = 2
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

func buildUnsolicitedNA(ip net.IP, mac net.HardwareAddr) []byte {
	b := make([]byte, 32)
	b[0] = icmpTypeNA
	b[4] = naFlagOverride
	copy(b[8:24], ip.To16())
	b[24] = optTargetLLAddr
	b[25] = 1
	copy(b[26:32], mac)
	return b
}

var errOpenNetns = errors.New("neighbor: open netns")

func buildFor(ip net.IP, mac net.HardwareAddr) ([]byte, bool) {
	if v4 := ip.To4(); v4 != nil {
		return buildGratuitousARP(v4, mac), false
	}
	return buildUnsolicitedNA(ip, mac), true
}

// Announce enters the container netns on a locked, never-unlocked OS thread
// (so the CLONE_NEWNET-contaminated thread is discarded) and sends 2x each
// gratuitous ARP (v4) / unsolicited NA (v6).
func Announce(netnsPath, iface string, addrs []net.IP, mac net.HardwareAddr) error {
	if len(addrs) == 0 {
		return nil
	}
	if _, err := os.Stat(netnsPath); err != nil {
		return fmt.Errorf("%w: %s: %v", errOpenNetns, netnsPath, err)
	}
	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // deliberately never unlocked
		fd, err := unix.Open(netnsPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
		if err != nil {
			errc <- fmt.Errorf("%w: %s: %v", errOpenNetns, netnsPath, err)
			return
		}
		defer unix.Close(fd)
		if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
			errc <- fmt.Errorf("neighbor: setns: %w", err)
			return
		}
		errc <- announceInNS(iface, addrs, mac)
	}()
	return <-errc
}

func announceInNS(iface string, addrs []net.IP, mac net.HardwareAddr) error {
	for _, ip := range addrs {
		pkt, isV6 := buildFor(ip, mac)
		var err error
		if isV6 {
			err = sendNA(iface, pkt)
		} else {
			err = sendARP(iface, pkt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func htons(v uint16) uint16 { return (v<<8)&0xff00 | v>>8 }

func sendARP(iface string, pkt []byte) error {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("neighbor: iface %s: %w", iface, err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ARP)))
	if err != nil {
		return fmt.Errorf("neighbor: AF_PACKET: %w", err)
	}
	defer unix.Close(fd)
	addr := &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ARP), Ifindex: ifi.Index, Halen: 6}
	copy(addr.Addr[:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	for i := 0; i < 2; i++ {
		if err := unix.Sendto(fd, pkt, 0, addr); err != nil {
			return fmt.Errorf("neighbor: send ARP: %w", err)
		}
	}
	return nil
}

func sendNA(iface string, body []byte) error {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("neighbor: iface %s: %w", iface, err)
	}
	c, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return fmt.Errorf("neighbor: icmp6 listen: %w", err)
	}
	defer c.Close()
	p := c.IPv6PacketConn()
	if err := p.SetHopLimit(255); err != nil {
		return fmt.Errorf("neighbor: set hop limit: %w", err)
	}
	if err := p.SetMulticastHopLimit(255); err != nil {
		return fmt.Errorf("neighbor: set multicast hop limit: %w", err)
	}
	dst := &net.IPAddr{IP: net.ParseIP("ff02::1"), Zone: iface}
	cm := &ipv6.ControlMessage{IfIndex: ifi.Index, HopLimit: 255}
	for i := 0; i < 2; i++ {
		if _, err := p.WriteTo(body, cm, dst); err != nil {
			return fmt.Errorf("neighbor: send NA: %w", err)
		}
	}
	return nil
}
