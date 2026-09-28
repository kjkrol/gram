package navigation

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/uid"
)

// body is a unit as BodySpacing sees it this tick: its box, how it moves and whether it stands.
type body struct {
	id       uid.UID64
	at       geom.Vec // the middle of its box
	half     geom.Vec // half its box
	vel      geom.Vec // world units a second
	domain   board.Domain
	moving   bool // under an order
	yielding bool // under an order to give way, lingering aside
}

// bodyIndex is the tick's bodies, by id and bucketed by the cells their boxes touch: what a unit
// learns of whoever it struck, and what a unit walked by hand feels just ahead.
type bodyIndex struct {
	grid    board.Grid
	bodies  []body
	byID    map[uid.UID64]int
	boxes   [][]int32 // per cell ordinal, the bodies whose box touches the cell
	touched []int     // the ordinals filled, emptied on the next build
	seen    []uint32  // per body, the stamp of the last near that met it
	stamp   uint32
	largest float64 // the largest half side of any body
}

// build indexes bodies, which the index keeps.
func (x *bodyIndex) build(bodies []body) {
	for _, o := range x.touched {
		x.boxes[o] = x.boxes[o][:0]
	}
	x.touched = x.touched[:0]
	if n := x.grid.CellCount(); len(x.boxes) != n {
		x.boxes = make([][]int32, n)
	}
	if x.byID == nil {
		x.byID = map[uid.UID64]int{}
	}
	clear(x.byID)
	x.bodies, x.largest = bodies, 0
	for i, b := range x.bodies {
		x.byID[b.id] = i
		x.largest = max(x.largest, b.half.X, b.half.Y)
		x.grid.CellsUnder(boxAt(b.at, b.half), func(c board.CellID) {
			if o, ok := x.grid.Ordinal(c); ok {
				if len(x.boxes[o]) == 0 {
					x.touched = append(x.touched, o)
				}
				x.boxes[o] = append(x.boxes[o], int32(i))
			}
		})
	}
	x.seen = append(x.seen[:0], make([]uint32, len(bodies))...)
}

// of is the body id, if it was there this tick.
func (x *bodyIndex) of(id uid.UID64) (body, bool) {
	i, ok := x.byID[id]
	if !ok {
		return body{}, false
	}
	return x.bodies[i], true
}

// near calls fn once with every body whose box touches a cell box touches.
func (x *bodyIndex) near(box geom.AABB, fn func(b *body)) {
	if len(x.boxes) == 0 {
		return
	}
	x.stamp++
	if x.stamp == 0 {
		clear(x.seen)
		x.stamp = 1
	}
	x.grid.CellsUnder(box, func(c board.CellID) {
		if o, ok := x.grid.Ordinal(c); ok {
			for _, i := range x.boxes[o] {
				if x.seen[i] != x.stamp {
					x.seen[i] = x.stamp
					fn(&x.bodies[i])
				}
			}
		}
	})
}

// boxAt is the box of half size half round c.
func boxAt(c, half geom.Vec) geom.AABB {
	return geom.NewAABBAt(geom.NewVec(c.X-half.X, c.Y-half.Y), 2*half.X, 2*half.Y)
}

// apart reports whether boxes of half sizes ha round a and hb round b, d = b − a, leave gap
// between them on some axis.
func apart(d, ha, hb geom.Vec, gap float64) bool {
	return math.Abs(d.X) >= ha.X+hb.X+gap || math.Abs(d.Y) >= ha.Y+hb.Y+gap
}
