package isometry

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

var _ camera.Projection = projection{}

// projection is the 2:1 view of Transport Tycoon: a Cell-sized square of the world is a TileW x
// TileH diamond, the world's x axis runs down-right and its y axis down-left, and a height lifts a
// point HeightUnit screen units per world unit. Headroom is how far above the ground Visible looks
// for sprites (default 64). Zero TileW, TileH and HeightUnit are 64, 32 and 1.
type projection struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
}

// withDefaults fills the zero fields; Cell must be set.
func (p projection) withDefaults() projection {
	if p.Cell <= 0 {
		panic("isometry: the view needs the world size of a cell")
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

func (p projection) Project(x, y, z float32) (float32, float32) {
	u, v := x/p.Cell, y/p.Cell
	return (u - v) * p.TileW / 2, (u+v)*p.TileH/2 - z*p.HeightUnit
}

func (p projection) Unproject(sx, sy, z float32) (float32, float32) {
	sy += z * p.HeightUnit
	a, b := sx/(p.TileW/2), sy/(p.TileH/2)
	return (a + b) / 2 * p.Cell, (b - a) / 2 * p.Cell
}

// Depth is the diagonal row of the cell under the point: rows further back are smaller, and
// everything in one cell ties with its tile, so what a Composer is handed on a higher tier — the
// entities standing on it — is drawn over it and under the row in front.
func (p projection) Depth(x, y, _ float32) float32 {
	return float32(math.Floor(float64(x/p.Cell))) + float32(math.Floor(float64(y/p.Cell)))
}

func (projection) Wraps() bool { return false }
func (projection) Sorts() bool { return true }
