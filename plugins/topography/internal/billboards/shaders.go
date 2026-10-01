package billboards

import "embed"

// shaders is the billboards' and their shadows' shaders.
//
//go:embed shaders/*.wgsl
var shaders embed.FS
