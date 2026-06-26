package sqlutils

import "regexp"

// AsRegexp matches case-insensitive " AS " with optional whitespace.
var AsRegexp = regexp.MustCompile(`(?i)\s+as\s+`)
