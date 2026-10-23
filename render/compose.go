package render

import "github.com/kjkrol/gram/render/internal/compose"

// Frame gathers what the sources of a Composer draw for one viewport, in screen pixels, each
// piece where it lies (At), and where the Direct sources draw themselves.
type Frame = compose.Frame

// At is where a piece of the picture lies: at a height in the world, or on top of it as a mark.
type At = compose.At

// Source is what draws a part of the world by handing its pieces to a Frame.
type Source = compose.Source

// Overlying is a Source whose pieces lie over the figures' ground.
type Overlying = compose.Overlying

// Direct is a Source that draws a part of the picture itself, with a shader of its own.
type Direct = compose.Direct

// Glass is a Source drawing on the glass over its picture, after everything laid over it.
type Glass = compose.Glass

// Target is where a Direct source draws.
type Target = compose.Target

// Pass is which part of the picture a Direct draws in.
type Pass = compose.Pass

// The passes: the opaque ground first, then what is seen through, back to front.
const (
	Opaque     = compose.Opaque
	SeeThrough = compose.SeeThrough
)

// Uniforms is the composer's shader's uniforms this frame, by name, for a Direct source.
type Uniforms = compose.Uniforms

// Light is a colour of light, red, green and blue; 1 each is white.
type Light = compose.Light

// Shade is the light a piece is drawn in at its corners, blended across it.
type Shade = compose.Shade

// Taking is which shade a drawn thing takes, bits of it.
type Taking = compose.Taking

// The shades a drawn thing takes.
const (
	TakesRelief = compose.TakesRelief
	TakesBlocks = compose.TakesBlocks
	TakesClouds = compose.TakesClouds
	TakesLamps  = compose.TakesLamps
	Shines      = compose.Shines
	TakesAll    = compose.TakesAll
)

// World is where each corner of a piece lies in the world, in the order of its Corners.
type World = compose.World

// Overlay is a quad laid over a sprite for a material to work out in the shader.
type Overlay = compose.Overlay

// GroundHeights is the ground's heights for the shaders to read.
type GroundHeights = compose.GroundHeights

// GroundShade is the sun on the ground for the shaders to read.
type GroundShade = compose.GroundShade

// ShadeBands is how many bands of rows down the ground's shade fade on their own.
const ShadeBands = compose.ShadeBands

// Composer is a Picture drawing its Sources as one picture, laid in layers as a Laid one.
type Composer = compose.Composer

var (
	// Furthest is behind the whole world: the sky.
	Furthest = compose.Furthest
	// Nearest is before the whole world: what falls through the air.
	Nearest = compose.Nearest
)

var (
	_ Picture = (*Composer)(nil)
	_ Laid    = (*Composer)(nil)
)

// OnTop is a mark at level, not the world: drawn over it on the screen, a higher level over a
// lower, at one level in the order the marks came.
func OnTop(level float32) At { return compose.OnTop(level) }

// UniformsOf is the uniforms m, by name, each a []float32, or an *Image for a frame's image.
func UniformsOf(m map[string]any) Uniforms { return compose.UniformsOf(m) }

// NewComposer composes sources into one picture; a nil source is left out.
func NewComposer(sources ...Source) *Composer { return compose.NewComposer(sources...) }
