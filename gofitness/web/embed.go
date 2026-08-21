// Package web embeds the frontend assets into the binary so the add-on ships as
// a single file with no runtime dependencies.
package web

import "embed"

// Files holds everything under static/.
//
//go:embed static
var Files embed.FS
