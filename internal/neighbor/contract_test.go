// SPDX-License-Identifier: Apache-2.0
package neighbor

import "net"

var _ func(string, string, []net.IP, net.HardwareAddr) error = Announce
