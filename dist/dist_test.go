// SPDX-License-Identifier: Apache-2.0
package dist

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type ociHook struct {
	Hook struct {
		Path string   `json:"path"`
		Args []string `json:"args"`
	} `json:"hook"`
	When struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"when"`
	Stages []string `json:"stages"`
}

func load(t *testing.T, name string) ociHook {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var h ociHook
	if err := json.Unmarshal(data, &h); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return h
}

func TestCreateRuntimeHook(t *testing.T) {
	h := load(t, "oci-hook.json")
	if !reflect.DeepEqual(h.Stages, []string{"createRuntime"}) ||
		h.Hook.Path != "/usr/local/lib/xnet/oci-hook" ||
		!reflect.DeepEqual(h.Hook.Args, []string{"oci-hook", "createRuntime"}) ||
		h.When.Annotations["org.octanix.rootful_networks"] != ".+" {
		t.Fatalf("hook = %+v", h)
	}
}
func TestPoststopHook(t *testing.T) {
	h := load(t, "oci-hook-poststop.json")
	if !reflect.DeepEqual(h.Stages, []string{"poststop"}) ||
		!reflect.DeepEqual(h.Hook.Args, []string{"oci-hook", "poststop"}) {
		t.Fatalf("hook = %+v", h)
	}
}
