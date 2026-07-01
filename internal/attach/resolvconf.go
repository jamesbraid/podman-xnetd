// SPDX-License-Identifier: Apache-2.0
package attach

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"

	"go.podman.io/common/libnetwork/types"

	"github.com/jamesbraid/xnetd/internal/proto"
)

const resolvSearchDomain = "dns.podman"

// renderResolvConf builds resolv.conf: deduped nameservers in first-seen order
// (networks visited sorted for determinism) + the podman search domain.
func renderResolvConf(status map[string]types.StatusBlock) []byte {
	names := make([]string, 0, len(status))
	for n := range status {
		names = append(names, n)
	}
	sort.Strings(names)
	seen := map[string]struct{}{}
	var buf bytes.Buffer
	for _, n := range names {
		for _, ip := range status[n].DNSServerIPs {
			s := ip.String()
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			fmt.Fprintf(&buf, "nameserver %s\n", s)
		}
	}
	fmt.Fprintf(&buf, "search %s\n", resolvSearchDomain)
	return buf.Bytes()
}

// WriteResolvConf writes resolv.conf into the container via BOTH the user and
// mount namespaces (createRuntime is pre-pivot_root; overlay only visible in
// the user+mount ns; mount-only enter hits EOVERFLOW). No-op unless Pid>0 and
// RootfsPath set.
func WriteResolvConf(req proto.Request, status map[string]types.StatusBlock) error {
	if req.RootfsPath == "" || req.Pid <= 0 {
		return nil
	}
	target := filepath.Join(req.RootfsPath, "etc", "resolv.conf")
	cmd := exec.Command("nsenter",
		fmt.Sprintf("--user=/proc/%d/ns/user", req.Pid),
		fmt.Sprintf("--mount=/proc/%d/ns/mnt", req.Pid),
		"sh", "-c", fmt.Sprintf("cat > %s", target))
	cmd.Stdin = bytes.NewReader(renderResolvConf(status))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("attach: write resolv.conf: %w: %s", err, out)
	}
	return nil
}
