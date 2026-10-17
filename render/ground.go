package render

import "image"

// GroundHeights is the ground's heights for the shaders to read (WGSL groundHeightAt): a lattice
// of Corners corners Cell world units apart from the world's origin, each a pixel of Image from
// At, its height Low up to Low+Span in red and green as 16 bits.
type GroundHeights struct {
	Image     *Image
	At        image.Point
	Corners   [2]int
	Cell      float32
	Low, Span float32
}

func init() { RegisterImage("GroundHeights") }

// GroundHeights hands every shader drawing this frame the ground's heights g, as whoever draws
// the ground has them; a frame without them has no ground's heights.
func (f *Frame) GroundHeights(g GroundHeights) {
	at := g.Image.Bounds().Min.Add(g.At)
	f.Image("GroundHeights", g.Image)
	f.Uniform("GroundLattice", float32(g.Corners[0]), float32(g.Corners[1]), g.Cell, 1)
	f.Uniform("GroundSpan", float32(at.X), float32(at.Y), g.Low, g.Span)
}

// ShadeBands is how many bands of rows down the ground's shade fade on their own from the shade
// before them to their own.
const ShadeBands = 16

// GroundShade is the sun on the ground for the shaders to read (WGSL groundShadeOf,
// groundShadeAt, standingShadeAt): Now over the world box Area (origin and size), each of its
// ShadeBands bands Band of its height tall faded into from Was as far as Mix says. Its red is the
// sun reaching the ground, green and blue the roots of how high the relief's and the blocks'
// shadows reach over it as shares of Span (blue one less), alpha the sun the blocks leave.
type GroundShade struct {
	Now, Was *Image
	Area     [4]float32
	Span     float32
	Band     float32
	Mix      [ShadeBands]float32
}

func init() {
	RegisterImage("GroundShade")
	RegisterImage("GroundShadeWas")
}

var shadeMixes = [ShadeBands / 4]string{
	"GroundShadeMix0",
	"GroundShadeMix1",
	"GroundShadeMix2",
	"GroundShadeMix3",
}

// GroundShade hands every shader drawing this frame the ground's shade g, as whoever draws the
// ground bakes it; a frame without it has the sun reach everywhere.
func (f *Frame) GroundShade(g GroundShade) {
	f.Image("GroundShade", g.Now)
	f.Image("GroundShadeWas", g.Was)
	f.Uniform("GroundShadeArea", g.Area[:]...)
	f.Uniform("GroundShadeSpan", g.Span, g.Band)
	for k, name := range shadeMixes {
		f.Uniform(name, g.Mix[4*k:4*k+4]...)
	}
}
