package instanced

import (
	"github.com/kjkrol/gram/render/internal/compose"
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/text"
)

// the names of compose's, gfx's and text's the sprites and labels draw with
type (
	At              = compose.At
	Light           = compose.Light
	Taking          = compose.Taking
	Target          = compose.Target
	Uniforms        = compose.Uniforms
	AtlasSource     = gfx.AtlasSource
	SpriteID        = gfx.SpriteID
	DrawMeshOptions = gfx.DrawMeshOptions
	Font            = text.Font
)

// TakesAll takes every shade and the lamps' light, as a zero Taking does.
const TakesAll = compose.TakesAll
