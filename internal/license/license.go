// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 James Braid

// Package license enforces the repo's SPDX-first-line convention.
package license

import "bytes"

const spdxLine = "// SPDX-License-Identifier: Apache-2.0"

// HasSPDXHeader reports whether src begins with the mandatory SPDX line.
func HasSPDXHeader(src []byte) bool {
	if len(src) == 0 {
		return false
	}
	first := src
	if nl := bytes.IndexByte(src, '\n'); nl >= 0 {
		first = src[:nl]
	}
	return string(first) == spdxLine
}
