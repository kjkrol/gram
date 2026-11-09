package sector

import (
	"math/bits"

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
	// truncated, not floored: a point short of zero is held to the first sector either way
	c := min(max(int(x/g.side), 0), g.cols-1)
	r := min(max(int(y/g.side), 0), g.rows-1)
	return r*g.cols + c
}

// Box is sector i's rectangle, the last in a row or a column cut at the world's edge.
func (g *Grid) Box(i int) geom.AABB {
	c, r := float32(i%g.cols), float32(i/g.cols)
	return geom.NewAABB(geom.NewVec(c*g.side, r*g.side),
		geom.NewVec(min((c+1)*g.side, g.width), min((r+1)*g.side, g.height)))
}

// Tiers is how many tiers of cells lie over the sectors: tier 0 the sectors themselves, tier k
// cells of 2^k by 2^k sectors from sector 0, the top the first whose one cell covers the world.
func (g *Grid) Tiers() int { return bits.Len(uint(max(g.cols, g.rows)-1)) + 1 }

// TierLen is how many cells tier k has.
func (g *Grid) TierLen(k int) int { return g.tierCols(k) * g.tierRows(k) }

func (g *Grid) tierCols(k int) int { return (g.cols + 1<<k - 1) >> k }

func (g *Grid) tierRows(k int) int { return (g.rows + 1<<k - 1) >> k }

// TierBox is cell i of tier k's rectangle, cut at the world's edges.
func (g *Grid) TierBox(k, i int) geom.AABB {
	n, span := g.tierCols(k), g.side*float32(int(1)<<k)
	c, r := float32(i%n), float32(i/n)
	return geom.NewAABB(geom.NewVec(c*span, r*span),
		geom.NewVec(min((c+1)*span, g.width), min((r+1)*span, g.height)))
}

// TierOf is the cell of tier k holding the world point (x, y), held to the world's edges.
func (g *Grid) TierOf(k int, x, y float32) int {
	s := g.Of(x, y)
	return ((s/g.cols)>>k)*g.tierCols(k) + (s%g.cols)>>k
}

// Under calls each with every sector of cell i of tier k, row by row; one of tier 0 is the
// sector i.
func (g *Grid) Under(k, i int, each func(sector int)) {
	n := g.tierCols(k)
	c0, r0 := (i%n)<<k, (i/n)<<k
	c1, r1 := min(c0+1<<k, g.cols), min(r0+1<<k, g.rows)
	for r := r0; r < r1; r++ {
		for c := c0; c < c1; c++ {
			each(r*g.cols + c)
		}
	}
}

// Fit is the tier and the cell a thing lying over box lives in: one whose longer side is small at
// most (zero, a quarter of a sector's side) the sector of its centre, whatever it overlaps; a
// longer one the cell of the lowest tier holding it whole, the top's where none below does.
func (g *Grid) Fit(box geom.AABB, small float32) (tier, cell int) {
	if small <= 0 {
		small = g.side / 4
	}
	cx := (box.TopLeft.X + box.BottomRight.X) / 2
	cy := (box.TopLeft.Y + box.BottomRight.Y) / 2
	longer := max(box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)
	if longer <= small {
		return 0, g.Of(cx, cy)
	}
	top := g.Tiers() - 1
	// the tier to start from, the lowest whose cells (2^k sectors a side) are as long as the box
	n := int(f32.Ceil(min(longer/g.side, float32(int(1)<<top))))
	// a box past an edge the world does not wrap at is held by what of it lies within; one across
	// a seam it wraps at no cell short of the top holds
	lo, hi := box.TopLeft, box.BottomRight
	if !g.wrapX {
		lo.X, hi.X = max(lo.X, 0), min(hi.X, g.width)
	}
	if !g.wrapY {
		lo.Y, hi.Y = max(lo.Y, 0), min(hi.Y, g.height)
	}
	for k := bits.Len(uint(n - 1)); k < top; k++ {
		i := g.TierOf(k, cx, cy)
		if c := g.TierBox(k, i); c.TopLeft.X <= lo.X && c.TopLeft.Y <= lo.Y &&
			hi.X <= c.BottomRight.X && hi.Y <= c.BottomRight.Y {
			return k, i
		}
	}
	return top, 0
}

// Part is a sector a rectangle covers and the part of the rectangle within it.
type Part struct {
	Sector int
	Box    geom.AABB
}

// Cover appends to dst each sector box covers, once, with the part of box within it; where the
// world wraps, box may run past its edges, its part across the seam given where that sector lies.
func (g *Grid) Cover(box geom.AABB, dst []Part) []Part {
	from := len(dst)
	dst = g.axis(dst, box.TopLeft.X, box.BottomRight.X, g.cols, g.width, g.wrapX)
	xs := len(dst)
	dst = g.axis(dst, box.TopLeft.Y, box.BottomRight.Y, g.rows, g.height, g.wrapY)
	ys := len(dst)
	for j := xs; j < ys; j++ {
		for i := from; i < xs; i++ {
			x, y := dst[i].Box, dst[j].Box
			dst = append(dst, Part{
				Sector: dst[j].Sector*g.cols + dst[i].Sector,
				Box: geom.NewAABB(geom.NewVec(x.TopLeft.X, y.TopLeft.X),
					geom.NewVec(x.BottomRight.X, y.BottomRight.X)),
			})
		}
	}
	return dst[:from+copy(dst[from:], dst[ys:])]
}

// appends to dst the slots of an axis of n, size long, that [lo, hi) covers, each once, as a Part
// of the slot whose box spans the stretch of the axis along X alone
func (g *Grid) axis(dst []Part, lo, hi float32, n int, size float32, wraps bool) []Part {
	switch {
	case !wraps:
		return g.slots(dst, max(lo, 0), min(hi, size), n, size)
	case hi-lo >= size:
		return g.slots(dst, 0, size, n, size)
	}
	shift := f32.Floor(lo/size) * size
	lo, hi = lo-shift, hi-shift
	if hi <= size {
		return g.slots(dst, lo, hi, n, size)
	}
	// what runs past the seam ends short of lo: a slot holding both ends is one, whole
	first := len(dst)
	dst = g.slots(dst, 0, hi-size, n, size)
	at := len(dst)
	dst = g.slots(dst, lo, size, n, size)
	if at > first && at < len(dst) && dst[at-1].Sector == dst[at].Sector {
		dst[at-1].Box.BottomRight.X = dst[at].Box.BottomRight.X
		dst = append(dst[:at], dst[at+1:]...)
	}
	return dst
}

func (g *Grid) slots(dst []Part, lo, hi float32, n int, size float32) []Part {
	for k := int(lo / g.side); k < n && float32(k)*g.side < hi; k++ {
		from, to := max(lo, float32(k)*g.side), min(hi, float32(k+1)*g.side, size)
		if from < to {
			dst = append(dst, Part{Sector: k,
				Box: geom.NewAABB(geom.NewVec(from, 0), geom.NewVec(to, 0))})
		}
	}
	return dst
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
