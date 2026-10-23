package compose

import (
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

// the names of gfx's compose draws with
type (
	Image       = gfx.Image
	Depth       = gfx.Depth
	Vertex      = gfx.Vertex
	AtlasSource = gfx.AtlasSource
	SpriteID    = gfx.SpriteID
	Corners     = gfx.Corners
	Fade        = gfx.Fade
	MaterialID  = wgsl.MaterialID
)
