package camera

import "math"

// Projection is how a world point at a height lands on the screen at zoom 1, with the world's
// origin at the screen's: the mapping a Camera draws through, pure arithmetic without a window.
type Projection interface {
	// Project maps the world point (x, y) at height z to screen units.
	Project(x, y, z float32) (sx, sy float32)
	// Unproject inverts Project at height z: the world point drawn at (sx, sy).
	Unproject(sx, sy, z float32) (x, y float32)
	// Depth orders drawing: what is further back is smaller and drawn first.
	Depth(x, y, z float32) float32
	// Wraps reports whether a world wrapping at its edges can be drawn through this projection.
	Wraps() bool
}

// TopDown is the plain map view: screen x and y are world x and y, height is not drawn.
type TopDown struct{}

func (TopDown) Project(x, y, _ float32) (float32, float32)     { return x, y }
func (TopDown) Unproject(sx, sy, _ float32) (float32, float32) { return sx, sy }
func (TopDown) Depth(_, y, _ float32) float32                  { return y }
func (TopDown) Wraps() bool                                    { return true }

// Isometric is the 2:1 view of Transport Tycoon: a Cell-sized square of the world is a TileW x
// TileH diamond, the world's x axis runs down-right and its y axis down-left, and a height lifts a
// point HeightUnit screen units per world unit. Headroom is how far above the ground Visible looks
// for sprites (default 64). Zero TileW, TileH and HeightUnit are 64, 32 and 1.
type Isometric struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
}

// WithDefaults fills the zero fields; Cell must be set.
func (p Isometric) WithDefaults() Isometric {
	if p.Cell <= 0 {
		panic("camera: Isometric needs the world size of a cell")
	}
	if p.TileW == 0 {
		p.TileW = 64
	}
	if p.TileH == 0 {
		p.TileH = 32
	}
	if p.HeightUnit == 0 {
		p.HeightUnit = 1
	}
	if p.Headroom == 0 {
		p.Headroom = 64
	}
	return p
}

func (p Isometric) Project(x, y, z float32) (float32, float32) {
	u, v := x/p.Cell, y/p.Cell
	return (u - v) * p.TileW / 2, (u+v)*p.TileH/2 - z*p.HeightUnit
}

func (p Isometric) Unproject(sx, sy, z float32) (float32, float32) {
	sy += z * p.HeightUnit
	a, b := sx/(p.TileW/2), sy/(p.TileH/2)
	return (a + b) / 2 * p.Cell, (b - a) / 2 * p.Cell
}

// Depth is the diagonal row of the cell under the point: rows further back are smaller, and
// everything in one cell ties with its tile, so what a Sorted layer submits after the terrain —
// the entities standing on it — is drawn over it and under the row in front.
func (p Isometric) Depth(x, y, _ float32) float32 {
	return float32(math.Floor(float64(x/p.Cell))) + float32(math.Floor(float64(y/p.Cell)))
}

func (Isometric) Wraps() bool { return false }
