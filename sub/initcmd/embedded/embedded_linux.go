package embedded

import "embed"

//go:embed mercury_intercept.so mercury_tun scripts
var FS embed.FS
