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
