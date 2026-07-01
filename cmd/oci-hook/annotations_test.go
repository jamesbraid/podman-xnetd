// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseAnnotations(t *testing.T) {
	const fixture = `{"org.octanix.rootful_networks":"lan, dmz ","org.octanix.static_ip.lan":"10.0.0.5, 10.0.0.6","org.octanix.static_ip.dmz":"192.168.9.9","org.octanix.container_name":" web ","unrelated":"x"}`
	var ann map[string]string
	json.Unmarshal([]byte(fixture), &ann)
	nets, ips, name := parseAnnotations(ann)
	if !reflect.DeepEqual(nets, []string{"lan", "dmz"}) || name != "web" ||
		!reflect.DeepEqual(ips, map[string][]string{"lan": {"10.0.0.5", "10.0.0.6"}, "dmz": {"192.168.9.9"}}) {
		t.Fatalf("nets=%v ips=%v name=%q", nets, ips, name)
	}
}

func TestParseAnnotationsEmpty(t *testing.T) {
	nets, ips, name := parseAnnotations(map[string]string{})
	if len(nets) != 0 || len(ips) != 0 || name != "" {
		t.Fatalf("want empty")
	}
}
