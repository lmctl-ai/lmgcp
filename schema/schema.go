// SPDX-License-Identifier: MIT

package schema

import (
	_ "embed"
)

// EmbeddedSchema contains the raw bytes of the lmgcp DSL schema.
//
//go:embed lmgcp.dsl.json
var EmbeddedSchema []byte
