package painter

import (
	"math"

	"github.com/kjkrol/gram/plugins/topography/water"
	"github.com/kjkrol/gram/render"
)

// Shine is how shiny the tile's top is — its kind's Shine, as far as its Detail — and how much of
// the sun reaches each of its corners, none at night; false where the tile does not shine at all: a
// kind without shine, a flat world, a tile too far off. A Look hands them to [Glint]
// after the top's sprite.
func (t *tile) Shine() (shine float32, lit [4]float32, ok bool) {
	shine = t.baseTop().shine * t.Detail()
	if shine <= 0 || !t.r.heights {
		return 0, lit, false
	}
	return shine, t.sunlit(), true
}

// Flow is how fast the water on the tile runs at each of its corners: down the slope of each cell
// of running water meeting there, as fast as its kind's Flow by the square root of the slope,
// the cells' runs averaged; banks that do not run count for nothing, so the current neither turns
// into them nor breaks between two tiles. False where the tile's water is still, off a square grid.
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

// runOf is how fast water runs over a cell whose corners stand at g, w by h, running at flow down a
// slope of 1 in 1: down its slope, by the square root of it.
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

// Detail is how much of what is fine the tile is drawn with, 0 to 1, by how many pixels it spans:
// all of it from nearCell up, none of it — no glint, no running water over it — from farCell
// down, so a far view of the whole world draws no more than its tiles and what shows from afar.
func (t *tile) Detail() float32 {
	return min(max((t.px()-farCell)/(nearCell-farCell), 0), 1)
}

// The pixels a cell spans from which a tile is drawn with no detail, all of it, and its shore.
const (
	farCell   = 6
	nearCell  = 12
	shoreCell = 16
)

// DrawSurface lays over the tile's top just drawn, its box x0..x1, y0..y1, where it shines its
// water, running or still.
func (t *tile) DrawSurface(f *render.Frame, x0, y0, x1, y1 float32) {
	box := render.Box(x0, y0, x1, y1)
	shine, lit, ok := t.Shine()
	if !ok {
		return
	}
	even := [4]float32{shine, shine, shine, shine}
	if flow, ok := t.Flow(); ok {
		water.Stream(f, box, even, lit, flow)
	} else {
		water.Glint(f, box, even, lit, t.Shore())
	}
}
