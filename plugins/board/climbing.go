package board

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Climbing is what slopes do to whoever goes over them, by the slope: rise over run. A climb takes
// 1 + Up·slope times as long as the flat. A gentle descent is quicker, the quickest — 1 − Down
// times as long, Down below 1 — at a fall of Ease; a steeper one slows again by Steep for every
// unit of fall past Ease: a steep way down is picked carefully. Whoever moves in a Free domain
// flies over.
type Climbing struct {
	Up, Down    float64
	Ease, Steep float64
	Free        Domain
}

// DefaultClimbing has a climb of 1 in 10 take twice as long as the flat, a descent of 1 in 10 the
// quickest, 0.7 as long, one of 1 in 5 slower than the flat, and Air fly over.
var DefaultClimbing = Climbing{Up: 10, Down: 0.3, Ease: 0.1, Steep: 5, Free: Air}

// Factor is how many times as long a step over slope takes as one on the flat.
func (c Climbing) Factor(slope float64) float64 {
	if slope >= 0 {
		return 1 + c.Up*slope
	}
	fall := -slope
	switch {
	case c.Ease <= 0:
		return 1 + c.Steep*fall // no descent is quick
	case fall <= c.Ease:
		return 1 - c.Down*fall/c.Ease
	}
	return 1 - c.Down + c.Steep*(fall-c.Ease)
}

// Least is the smallest Factor, on a descent of Ease.
func (c Climbing) Least() float64 {
	if c.Ease <= 0 {
		return 1
	}
	return min(1-c.Down, 1)
}

// Feels reports whether an entity moving in d climbs: none of its domains is Free.
func (c Climbing) Feels(d Domain) bool { return d&c.Free == 0 }

// Climbing is how this board's slopes slow and speed whoever goes over them.
func (b *Board) Climbing() Climbing { return b.climbing }

// SetClimbing sets how this board's slopes slow and speed whoever goes over them.
func (b *Board) SetClimbing(c Climbing) { b.climbing = c }

// Climb is how many times as long the step from a to its neighbour to takes an entity moving in d
// as it would on the flat: on a square grid the slope of to's ground the way the step goes, read
// off its corners; on a hex grid, whose cells are level, the rise between the two over the way
// between their middles.
func (b *Board) Climb(from, to CellID, d Domain) float64 {
	if !b.climbing.Feels(d) {
		return 1
	}
	if sq := b.square; sq != nil {
		fx, fy := sq.cellXY(from)
		tx, ty := sq.cellXY(to)
		dx, dy := stepAxis(fx, tx, sq.Width), stepAxis(fy, ty, sq.Height)
		w, h := b.CellBounds()
		c := b.Relief(to).Corners
		rise := (dx*float64(c[1]-c[0]+c[3]-c[2]) + dy*float64(c[2]-c[0]+c[3]-c[1])) / 2
		run := math.Hypot(dx*w, dy*h)
		if run <= 0 {
			return 1
		}
		return b.climbing.Factor(rise / run)
	}
	run := b.NeighborCost(from, to) * b.Step()
	if run <= 0 {
		return 1
	}
	return b.climbing.Factor((b.Altitude(to) - b.Altitude(from)) / run)
}

// stepAxis is the step from a to its neighbour b along an axis size long, -1, 0 or 1, across the
// seam where the grid wraps.
func stepAxis(a, b, size uint32) float64 {
	switch {
	case b == a:
		return 0
	case b == a+1 || a == size-1 && b == 0:
		return 1
	default:
		return -1
	}
}

// slopeAt is the ground's slope at p going dir (of length 1): on a square grid the rise of the
// cell's ground under p that way, between its corners; on a hex grid the rise across a cell's step
// round p, over that step.
func (b *Board) slopeAt(p, dir geom.Vec) float64 {
	if sq := b.square; sq != nil {
		c, ok := b.CellAt(p)
		if !ok {
			return 0
		}
		w, h := b.CellBounds()
		x, y := sq.cellXY(c)
		u, v := p.X/w-float64(x), p.Y/h-float64(y)
		u, v = u-math.Floor(u), v-math.Floor(v) // a wrapped p lies a whole board away
		k := b.Relief(c).Corners
		gx := ((1-v)*float64(k[1]-k[0]) + v*float64(k[3]-k[2])) / w
		gy := ((1-u)*float64(k[2]-k[0]) + u*float64(k[3]-k[1])) / h
		return gx*dir.X + gy*dir.Y
	}
	half := b.Step() / 2
	if half <= 0 {
		return 0
	}
	ahead := b.GroundAt(geom.NewVec(p.X+dir.X*half, p.Y+dir.Y*half))
	behind := b.GroundAt(geom.NewVec(p.X-dir.X*half, p.Y-dir.Y*half))
	return (ahead - behind) / (2 * half)
}
