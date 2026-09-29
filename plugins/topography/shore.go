package topography

import (
	"math"

	"github.com/kjkrol/gram/plugins/topography/heightfield"
)

// Shore is the way from each corner of the tile's top to the nearest cell within a few that does not
// shine — the shore of the water the tile is part of — how far it is and how near; open water
// beyond, and on a grid other than square. A Look hands it to [Glint] with the Shine.
// Too far off for a surf to show, it is open water all round.
func (t *tile) Shore() Shore {
	if t.px() < shoreCell {
		return Shore{}
	}
	return t.r.shoreOf(t.X0, t.Y0, t.X1, t.Y1)
}

// cornerShore is the shore as one corner of the grid sees it.
type cornerShore struct {
	corner ShoreCorner
	stamp  uint32
}

// nextShores starts a Compose: when the terrain has changed since the last one, every shore is
// worked out anew as it comes into sight.
func (l *dresser) nextShores() {
	if !l.square {
		return
	}
	version := l.version()
	if n := int(l.sq.Cols+1) * int(l.sq.Rows+1); len(l.shores) != n {
		l.shores, l.shoreStamp = make([]cornerShore, n), 0
	} else if version == l.shoreFor && l.shoreStamp != 0 {
		return
	}
	l.shoreFor = version
	if l.shoreStamp++; l.shoreStamp == 0 { // wrapped round: old results would pass for new
		clear(l.shores)
		l.shoreStamp = 1
	}
}

// shoreReach is how many cells from a shore its waves turn to face it.
const shoreReach = 3

// shoreOf is the shore from each corner of the box x0..x1, y0..y1 of a cell: open water everywhere
// off a square grid.
func (l *dresser) shoreOf(x0, y0, x1, y1 float32) Shore {
	if !l.square {
		return Shore{}
	}
	return Shore{l.shoreAt(x0, y0), l.shoreAt(x1, y0), l.shoreAt(x0, y1), l.shoreAt(x1, y1)}
}

// shoreAt is the shore from the grid's corner at (x, y), worked out the first time it is asked
// for since the terrain changed.
func (l *dresser) shoreAt(x, y float32) ShoreCorner {
	sq := l.sq
	size := float32(sq.Cell)
	gx, gy := int64(math.Round(float64(x/size))), int64(math.Round(float64(y/size)))
	slot, ok := cornerSlot(gx, int64(sq.Cols), sq.WrapX)
	row, okY := cornerSlot(gy, int64(sq.Rows), sq.WrapY)
	if !ok || !okY {
		return l.workShore(gx, gy, size)
	}
	s := &l.shores[row*(int64(sq.Cols)+1)+slot]
	if s.stamp != l.shoreStamp {
		s.corner, s.stamp = l.workShore(gx, gy, size), l.shoreStamp
	}
	return s.corner
}

// cornerSlot is corner g of n cells along an axis, folded onto 0..n-1 when it wraps; false off the
// grid.
func cornerSlot(g, n int64, wrap bool) (int64, bool) {
	if wrap {
		return (g%n + n) % n, true
	}
	return g, g >= 0 && g <= n
}

// workShore is the way from the grid's corner gx, gy to the nearest cell within shoreReach that
// does not shine, how far it is and how near; on the shore itself, the way into the land it
// touches.
func (l *dresser) workShore(gx, gy int64, size float32) ShoreCorner {
	reach := shoreReach * size
	px, py := float32(gx)*size, float32(gy)*size
	best, bx, by := reach*reach, float32(0), float32(0)
	var ax, ay float32 // on the shore: towards the middles of the land cells touching the corner
	look := func(cx, cy int64) {
		c, ok := l.cellAt(cx, cy)
		if !ok || l.topOf(c).shine > 0 {
			return
		}
		x0, y0 := float32(cx)*size, float32(cy)*size
		dx, dy := min(max(px, x0), x0+size)-px, min(max(py, y0), y0+size)-py
		d := dx*dx + dy*dy
		if d == 0 {
			ax, ay = ax+x0+size/2-px, ay+y0+size/2-py
		}
		if d < best {
			best, bx, by = d, dx, dy
		}
	}
	// ring r is the cells r away from the four touching the corner, none of them nearer than r cells
	for r := int64(0); r <= shoreReach; r++ {
		if near := float32(r) * size; near*near >= best {
			break
		}
		lo, hi := -1-r, r
		for cx := lo; cx <= hi; cx++ {
			look(gx+cx, gy+lo)
			look(gx+cx, gy+hi)
		}
		for cy := lo + 1; cy < hi; cy++ {
			look(gx+lo, gy+cy)
			look(gx+hi, gy+cy)
		}
	}
	if best >= reach*reach {
		return ShoreCorner{Dist: reach}
	}
	d := float32(math.Sqrt(float64(best)))
	if d == 0 {
		bx, by = ax, ay
	}
	corner := ShoreCorner{Dist: d, Near: 1 - d/reach}
	if n := float32(math.Hypot(float64(bx), float64(by))); n > 0 {
		corner.X, corner.Y = bx/n, by/n
	}
	return corner
}

// coast is the way to the shore from every corner of a square grid, row by row, for the ground
// traced on the GPU: worked out anew only round the cells whose shine has changed.
type coast struct {
	shining []bool // by ordinal, whether the cell's own kind shines
	shores  []heightfield.Shore
	seen    uint64 // one more than the board's count of changes when last brought up to date
	version uint64 // counts the changes to shores
}

// Coast is the way to the shore from every corner of the square grid, row by row, how far off a
// shore is seen, and a count of the changes: brought up to date with the board, round the cells
// that began or ceased to shine alone — not with the relief, which moves no shore; nil off a
// square grid.
func (l *dresser) Coast() ([]heightfield.Shore, float32, uint64) {
	if !l.square {
		return nil, 0, 0
	}
	c := &l.coast
	size := float32(l.sq.Cell)
	reach := shoreReach * size
	if c.seen == l.board.Changes()+1 {
		return c.shores, reach, c.version
	}
	c.seen = l.board.Changes() + 1
	l.tables()
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	full := len(c.shining) != cols*rows || len(c.shores) != (cols+1)*(rows+1)
	if full {
		c.shining, c.shores = make([]bool, cols*rows), make([]heightfield.Shore, (cols+1)*(rows+1))
	}
	work := func(gx, gy int) {
		c.shores[gy*(cols+1)+gx] = heightfield.Shore(l.workShore(int64(gx), int64(gy), size))
	}
	changed := full
	for i := range cols * rows {
		cell, _ := l.cellAt(int64(i%cols), int64(i/cols))
		now := l.topOf(cell).shine > 0
		if !full && now == c.shining[i] {
			continue
		}
		c.shining[i], changed = now, true
		if full {
			continue
		}
		x, y := i%cols, i/cols // the corners whose shore the cell may be
		for gy := max(0, y-shoreReach-1); gy <= min(rows, y+shoreReach+2); gy++ {
			for gx := max(0, x-shoreReach-1); gx <= min(cols, x+shoreReach+2); gx++ {
				work(gx, gy)
			}
		}
	}
	if full {
		for gy := 0; gy <= rows; gy++ {
			for gx := 0; gx <= cols; gx++ {
				work(gx, gy)
			}
		}
	}
	if changed {
		c.version++
	}
	return c.shores, reach, c.version
}
