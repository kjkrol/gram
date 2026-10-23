package painter

import (
	"math"

	"github.com/kjkrol/gram/plugins/topography/internal/water"
)

// Shine is how shiny the tile's top is and how much of the sun reaches each of its corners.
// It is false where the tile does not shine at all: a kind without shine, a flat world, a tile
// too far off.
func (t *tile) Shine() (shine float32, lit [4]float32, ok bool) {
	shine = t.baseTop().shine * t.Detail()
	if shine <= 0 || !t.r.heights {
		return 0, lit, false
	}
	return shine, t.sunlit(), true
}

// Flow is how fast the water on the tile runs at each of its corners. It is false where the
// tile's water is still, or off a square grid.
func (t *tile) Flow() (water.Flow, bool) {
	r := t.r
	if !r.square || t.baseTop().flow <= 0 {
		return water.Flow{}, false
	}
	x, y := r.xy(t.ID)
	w, h := r.board.CellBounds()
	var out water.Flow
	for k, d := range [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		n := 0
		for _, o := range [4][2]int64{{-1, -1}, {0, -1}, {-1, 0}, {0, 0}} {
			c, ok := r.cellAt(int64(x)+d[0]+o[0], int64(y)+d[1]+o[1])
			if !ok {
				continue
			}
			top := r.topOf(c)
			if top.flow <= 0 {
				continue
			}
			vx, vy := runOf(top.ground, top.flow, float32(w), float32(h))
			out[k][0], out[k][1], n = out[k][0]+vx, out[k][1]+vy, n+1
		}
		if n > 0 {
			out[k][0], out[k][1] = out[k][0]/float32(n), out[k][1]/float32(n)
		}
	}
	return out, true
}

// flow is the speed down a slope of 1 in 1
func runOf(g [4]float32, flow, w, h float32) (vx, vy float32) {
	gx := (g[1] - g[0] + g[3] - g[2]) / (2 * w)
	gy := (g[2] - g[0] + g[3] - g[1]) / (2 * h)
	slope := float32(math.Hypot(float64(gx), float64(gy)))
	if slope == 0 {
		return 0, 0
	}
	speed := flow * float32(math.Sqrt(float64(slope)))
	return -gx / slope * speed, -gy / slope * speed
}

// Detail is how much of what is fine the tile is drawn with, 0 to 1, by how many pixels it
// spans.
func (t *tile) Detail() float32 {
	return min(max((t.px()-farCell)/(nearCell-farCell), 0), 1)
}

// pixels a cell spans
const (
	farCell   = 6
	nearCell  = 12
	shoreCell = 16
)
