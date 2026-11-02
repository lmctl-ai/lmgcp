// SPDX-License-Identifier: MIT

package schema

import _ "embed"

// EmbeddedLMCloudSchema is the byte-identical shared lmcloud schema.
//
//go:embed lmcloud.dsl.json
var EmbeddedLMCloudSchema []byte
