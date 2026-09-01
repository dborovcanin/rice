// Package rice embeds the default templates and themes so a single binary is
// enough to generate a complete desktop configuration. User files in
// ~/.config/rice always take precedence over these defaults.
package rice

import "embed"

// Templates holds the built-in adapter templates, rooted at "templates/".
//
//go:embed templates
var Templates embed.FS

// Themes holds the bundled themes, rooted at "themes/".
//
//go:embed themes
var Themes embed.FS

// Version is the Rice version stamped into generation manifests.
const Version = "0.1.0"
