package topography

import (
	"embed"
	"math"

	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// seaGlint and runningWater are the materials of water on the board (shaders/): the sea's waves
// and surf, and water running down its slope; they call the clouds' functions (plugins/atmosphere/air).
var (
	waterMaterials = render.RegisterMaterials(render.Files(shaders, "shaders/water.wgsl", "shaders/sea.wgsl", "shaders/stream.wgsl"), nil, "SeaGlint", "RunningWater")
	seaGlint       = waterMaterials[0]
	runningWater   = waterMaterials[1]
)

// Shore is where the nearest shore lies from each corner of what glints — top-left, top-right,
// bottom-left, bottom-right; the zero Shore is open water.
type Shore [4]ShoreCorner

// ShoreCorner is the way to the nearest shore (X, Y, of length 1, or 0 with none near), how far
// it is in world units, and how near: 1 on the shore down to 0 where the open water begins.
type ShoreCorner struct{ X, Y, Dist, Near float32 }

// Flow is how fast water runs at each corner of what streams — top-left, top-right, bottom-left,
// bottom-right — in world units a second along x and y.
type Flow [4][2]float32

// Glint lays over the last sprite added to f, whose corners lie at w, water rippled by small waves
// the shader runs across it as time goes by, as shiny at each corner as shine, lit of the sun
// reaching each corner:
// it throws the sun back at the eye where it faces halfway between them and the sky the flatter the
// eye looks; near a shore the waves turn to face it and break into foam.
func Glint(f *render.Frame, w render.World, shine, lit [4]float32, shore Shore) {
	o := render.Overlay{Material: seaGlint, World: w, Red: shine, Fraction: lit}
	for k, c := range shore {
		o.Custom[k] = [4]float32{c.X, c.Y, c.Dist, c.Near}
	}
	f.Overlay(&o)
}

// Stream lays over the last sprite added to f, whose corners lie at w, water running at flow, as
// shiny at each corner as shine:
// ripples and flecks of foam carried down with the current, white where it runs fast — a rapid, a
// waterfall; over a sprite drawn blended it shows only where the sprite does.
func Stream(f *render.Frame, w render.World, shine, lit [4]float32, flow Flow) {
	o := render.Overlay{Material: runningWater, World: w, Red: shine, Fraction: lit, Blended: true}
	for k, v := range flow {
		o.Custom[k] = [4]float32{v[0], v[1], 0, 0}
	}
	f.Overlay(&o)
}

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
func (t *tile) Flow() (Flow, bool) {
	r := t.r
	if !r.square || t.baseTop().flow <= 0 {
		return Flow{}, false
	}
	x, y := r.xy(t.ID)
	w, h := r.board.CellBounds()
	var out Flow
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
	lit = t.r.shaded(lit, t.clouds()) // the clouds' shadow dims what the sun gives the water
	if flow, ok := t.Flow(); ok {
		Stream(f, box, even, lit, flow)
	} else {
		Glint(f, box, even, lit, t.Shore())
	}
}
