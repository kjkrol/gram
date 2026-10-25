package sector

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
)

// Grid is a world cut into square sectors, each in a band: its index among the bands, or Beyond
// past the last.
type Grid struct {
	side         float32
	cols, rows   int
	width        float32
	height       float32
	wrapX, wrapY bool
	bands        []Band
	in           []uint8
}

// NewGrid cuts a world width x height, wrapping across where wrapX says and down where wrapY
// does, into sectors side a side in bands; every sector begins Beyond.
func NewGrid(width, height uint32, wrapX, wrapY bool, side float32, bands []Band) *Grid {
	cols := max(int(f32.Ceil(float32(width)/side)), 1)
	rows := max(int(f32.Ceil(float32(height)/side)), 1)
	g := &Grid{side: side, cols: cols, rows: rows, width: float32(width),
		height: float32(height), wrapX: wrapX, wrapY: wrapY, bands: bands,
		in: make([]uint8, cols*rows)}
	g.Reset()
	return g
}

// Reset lays every sector Beyond again.
func (g *Grid) Reset() {
	for i := range g.in {
		g.in[i] = uint8(len(g.bands))
	}
}

// Len is how many sectors there are.
func (g *Grid) Len() int { return len(g.in) }

// Side is a sector's side, in world units.
func (g *Grid) Side() float32 { return g.side }

// Cols is how many sectors lie across the world; sector i lies in column i%Cols.
func (g *Grid) Cols() int { return g.cols }

// Rows is how many sectors lie down the world.
func (g *Grid) Rows() int { return g.rows }

// Beyond is the band of the sectors past the last band.
func (g *Grid) Beyond() int { return len(g.bands) }

// Of is the sector of the world point (x, y), held to the world's edges.
func (g *Grid) Of(x, y float32) int {
	c := min(max(int(f32.Floor(x/g.side)), 0), g.cols-1)
	r := min(max(int(f32.Floor(y/g.side)), 0), g.rows-1)
	return r*g.cols + c
}

// Box is sector i's rectangle, the last in a row or a column cut at the world's edge.
func (g *Grid) Box(i int) geom.AABB {
	c, r := float32(i%g.cols), float32(i/g.cols)
	return geom.NewAABB(geom.NewVec(c*g.side, r*g.side),
		geom.NewVec(min((c+1)*g.side, g.width), min((r+1)*g.side, g.height)))
}

// Band is the band sector i lies in, Beyond past the last.
func (g *Grid) Band(i int) int { return int(g.in[i]) }

// Count appends to dst how many sectors lie in each band, Beyond's last.
func (g *Grid) Count(dst []int) []int {
	start := len(dst)
	for range len(g.bands) + 1 {
		dst = append(dst, 0)
	}
	for _, b := range g.in {
		dst[start+int(b)]++
	}
	return dst
}

// Place lays every sector in the band its nearest anchor reaches, into a nearer band at once and
// into a farther one only margin past the reach it leaves; with no anchors, every sector in the
// first. changed hears of each sector moved, from its band to the new one.
func (g *Grid) Place(anchors []geom.Vec, margin float32, changed func(i, from, to int)) {
	for i := range g.in {
		from, to := int(g.in[i]), 0
		if len(anchors) > 0 {
			d := g.nearest(anchors, g.Box(i))
			if to = g.bandAt(d, 0); to > from {
				to = max(from, g.bandAt(d, margin))
			}
		}
		if to != from {
			g.in[i] = uint8(to)
			changed(i, from, to)
		}
	}
}

func (g *Grid) bandAt(d, margin float32) int {
	for k, b := range g.bands {
		if d <= b.Reach+margin {
			return k
		}
	}
	return len(g.bands)
}

// the distance from box to the nearest anchor, across a seam where the world wraps
func (g *Grid) nearest(anchors []geom.Vec, box geom.AABB) float32 {
	best := f32.Inf(1)
	for _, a := range anchors {
		dx := gap(a.X, box.TopLeft.X, box.BottomRight.X, g.width, g.wrapX)
		dy := gap(a.Y, box.TopLeft.Y, box.BottomRight.Y, g.height, g.wrapY)
		best = min(best, f32.Hypot(dx, dy))
	}
	return best
}

// how far x lies from [lo, hi], the shorter way round a wrapping axis of length size
func gap(x, lo, hi, size float32, wraps bool) float32 {
	d := max(lo-x, x-hi, 0)
	if !wraps {
		return d
	}
	x = f32.Mod(x, size)
	if x < 0 {
		x += size
	}
	for _, c := range [3]float32{x - size, x, x + size} {
		d = min(d, max(lo-c, c-hi, 0))
	}
	return d
}
