package painter

import (
	"image"
	"image/color"
)

// Style is how a kind of the board looks in relief beyond its sprite, set by the kind's name.
// Shine (0 to 1) is how much of the sun it throws back, Flow how fast water runs down its slope
// (world units a second at a fall of 1 in 1, 0 still), Spread (0 to a half) how softly it runs
// into a spreading neighbour, Under whether it lies under the kinds round it, MixWith the kind
// a way of it takes the look of towards its mouth, Detail the texture laid over its cells.
type Style struct {
	Shine   float32
	Flow    float32
	Spread  float32
	Under   bool
	MixWith string
	Detail  Detail
}

// DetailPx is the side a Detail's tile is baked at, in pixels; an Image of another size is
// resampled to it. MaxDetails is how many Details of their own a board's kinds may have.
// Dressed marks a cell whose pixels hold more than one kind, in the topography's own tables.
const (
	DetailPx   = 512
	MaxDetails = 48
	Dressed    = 255
)

// Detail is a texture laid over a kind's cells seen near, repeating every Across cells (1 where
// 0): its tone is taken as variation round the kind's own colour, so the colour stays the kind's
// and the texture gives the grain. The zero Detail is none.
type Detail struct {
	Image  image.Image
	Across float32
}

// Shading is how the relief's light is styled: Bands quantises the sun's light into that many
// steps (0 smooth), ShadowTint is the colour what lies in shade is tinted with (zero none), Warmth
// (0 to 1) how far the sun's colour is turned towards a warm one.
type Shading struct {
	Bands      int
	ShadowTint color.RGBA
	Warmth     float32
}
