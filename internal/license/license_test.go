// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 James Braid

package license

import "testing"

func TestHasSPDXHeader(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"exact first line", "// SPDX-License-Identifier: Apache-2.0\npackage x\n", true},
		{"missing", "package x\n", false},
		{"wrong license", "// SPDX-License-Identifier: MIT\npackage x\n", false},
		{"not first line", "// Copyright 2026\n// SPDX-License-Identifier: Apache-2.0\n", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasSPDXHeader([]byte(tc.src)); got != tc.want {
				t.Fatalf("HasSPDXHeader(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}
