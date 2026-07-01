// SPDX-License-Identifier: Apache-2.0
package main

import "strings"

const (
	annNetworks = "org.octanix.rootful_networks"
	annName     = "org.octanix.container_name"
	annIPPrefix = "org.octanix.static_ip."
)

func parseAnnotations(ann map[string]string) (networks []string, staticIPs map[string][]string, name string) {
	staticIPs = map[string][]string{}
	if v, ok := ann[annNetworks]; ok {
		networks = splitCSV(v)
	}
	if v, ok := ann[annName]; ok {
		name = strings.TrimSpace(v)
	}
	for k, v := range ann {
		if !strings.HasPrefix(k, annIPPrefix) {
			continue
		}
		if ips := splitCSV(v); len(ips) > 0 {
			staticIPs[strings.TrimPrefix(k, annIPPrefix)] = ips
		}
	}
	return networks, staticIPs, name
}

func splitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
