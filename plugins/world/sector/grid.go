package sector

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
)

// Grid is a world cut into square sectors.
type Grid struct {
	side         float32
	cols, rows   int
	width        float32
	height       float32
	wrapX, wrapY bool
}

// NewGrid cuts a world width x height, wrapping across where wrapX says and down where wrapY
// does, into sectors side a side.
func NewGrid(width, height uint32, wrapX, wrapY bool, side float32) *Grid {
	cols := max(int(f32.Ceil(float32(width)/side)), 1)
	rows := max(int(f32.Ceil(float32(height)/side)), 1)
	return &Grid{side: side, cols: cols, rows: rows, width: float32(width),
		height: float32(height), wrapX: wrapX, wrapY: wrapY}
}

// Len is how many sectors there are.
func (g *Grid) Len() int { return g.cols * g.rows }

// Side is a sector's side, in world units.
func (g *Grid) Side() float32 { return g.side }

// Cols is how many sectors lie across the world; sector i lies in column i%Cols.
func (g *Grid) Cols() int { return g.cols }

// Rows is how many sectors lie down the world.
func (g *Grid) Rows() int { return g.rows }

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

// Neighbours calls each with every sector touching sector i by a side or a corner, once, across a
// seam where the world wraps; one on the edge of a world that does not has fewer.
func (g *Grid) Neighbours(i int, each func(j int)) {
	c, r := i%g.cols, i/g.cols
	cols, nc := round(c, g.cols, g.wrapX)
	rows, nr := round(r, g.rows, g.wrapY)
	for _, rr := range rows[:nr] {
		for _, cc := range cols[:nc] {
			if rr != r || cc != c {
				each(rr*g.cols + cc)
			}
		}
	}
}

// k and the places either side of it on an axis of n, each once, round a seam where it wraps
func round(k, n int, wraps bool) (at [3]int, count int) {
	at[0], count = k, 1
	for _, d := range [2]int{-1, 1} {
		j := k + d
		if wraps {
			j = (j + n) % n
		}
		if j < 0 || j >= n || j == at[0] || count == 2 && j == at[1] {
			continue
		}
		at[count] = j
		count++
	}
	return at, count
}

// Nearest is the point of sector i nearest the world point (x, y) and how far it lies, the
// shorter way round where the world wraps.
func (g *Grid) Nearest(i int, x, y float32) (nx, ny, dist float32) {
	return g.nearest(g.Box(i), x, y)
}

// the point of box nearest (x, y) and its distance, across a seam where the world wraps
func (g *Grid) nearest(box geom.AABB, x, y float32) (nx, ny, dist float32) {
	dx, nx := gap(x, box.TopLeft.X, box.BottomRight.X, g.width, g.wrapX)
	dy, ny := gap(y, box.TopLeft.Y, box.BottomRight.Y, g.height, g.wrapY)
	return nx, ny, f32.Hypot(dx, dy)
}

// how far x lies from [lo, hi] and the point of it nearest, the shorter way round a wrapping axis
// of length size
func gap(x, lo, hi, size float32, wraps bool) (dist, near float32) {
	near = min(max(x, lo), hi)
	dist = f32.Abs(x - near)
	if !wraps {
		return dist, near
	}
	x = f32.Mod(x, size)
	if x < 0 {
		x += size
	}
	for _, c := range [3]float32{x - size, x, x + size} {
		if n := min(max(c, lo), hi); f32.Abs(c-n) < dist {
			dist, near = f32.Abs(c-n), n
		}
	}
	return dist, near
}
