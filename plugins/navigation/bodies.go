package navigation

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type body struct {
	id        uid.UID64
	at        geom.Vec // the middle of its box
	half      geom.Vec
	vel       geom.Vec // world units a second
	domain    cell.Domain
	moving    bool
	givingWay bool
	owners    tag.Tags[owner.Family]
	// the order's MoveTo, else the last it came to the end of; zero, none
	group     uint32
	goal      cell.ID
	spot      geom.Vec
	lastGoal  bool
	waitedOut bool
	cornered  bool
	minded    bool
	cell      cell.ID
	facing    geom.Vec // the way it faces, standing too
	z         world.Z
	lift      float64
}

func (b body) member() member {
	return member{id: b.id, cell: b.cell, from: b.cell, domain: b.domain, pos: world.Position{AABB: plane.NewAABB(geom.NewVec(b.at.X-b.half.X, b.at.Y-b.half.Y), 2*b.half.X, 2*b.half.Y)},
		vel: b.vel, facing: b.facing, z: b.z, lift: b.lift, owners: b.owners}
}

type bodyIndex struct {
	grid    grid.Grid
	bodies  []body
	byID    map[uid.UID64]int
	boxes   [][]int32 // per cell ordinal, the bodies whose box touches the cell
	touched []int
	largest float64
	adding  int32
	own     nearby
	add     func(cell.ID) // the method value, bound once
}

// nearby gathers the bodies of an index near a box in scratch of its own, for one caller at a
// time
type nearby struct {
	x     *bodyIndex
	found []int32
	seen  []uint32
	stamp uint32
	find  func(cell.ID) // the method value, bound once
}

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
	if x.add == nil {
		x.add = x.addCell
	}
	clear(x.byID)
	x.bodies, x.largest = bodies, 0
	for i, b := range x.bodies {
		x.byID[b.id] = i
		x.largest = max(x.largest, b.half.X, b.half.Y)
		x.adding = int32(i)
		x.grid.CellsUnder(boxAt(b.at, b.half), x.add)
	}
}

func (x *bodyIndex) addCell(c cell.ID) {
	if o, ok := x.grid.Ordinal(c); ok {
		if len(x.boxes[o]) == 0 {
			x.touched = append(x.touched, o)
		}
		x.boxes[o] = append(x.boxes[o], x.adding)
	}
}

func (x *bodyIndex) of(id uid.UID64) (body, bool) {
	i, ok := x.byID[id]
	if !ok {
		return body{}, false
	}
	return x.bodies[i], true
}

// near is the bodies whose box touches a cell box touches, each once, by their index in bodies;
// valid till the next call
func (x *bodyIndex) near(box geom.AABB) []int32 { return x.own.near(x, box) }

// near is x's near in n's scratch
func (n *nearby) near(x *bodyIndex, box geom.AABB) []int32 {
	n.x, n.found = x, n.found[:0]
	if len(x.boxes) == 0 {
		return n.found
	}
	if len(n.seen) < len(x.bodies) {
		n.seen = append(n.seen, make([]uint32, len(x.bodies)-len(n.seen))...)
	}
	n.stamp++
	if n.stamp == 0 {
		clear(n.seen)
		n.stamp = 1
	}
	if n.find == nil {
		n.find = n.findCell
	}
	x.grid.CellsUnder(box, n.find)
	return n.found
}

func (n *nearby) findCell(c cell.ID) {
	x := n.x
	if o, ok := x.grid.Ordinal(c); ok {
		for _, i := range x.boxes[o] {
			if n.seen[i] != n.stamp {
				n.seen[i] = n.stamp
				n.found = append(n.found, i)
			}
		}
	}
}

func boxAt(c, half geom.Vec) geom.AABB {
	return geom.NewAABBAt(geom.NewVec(c.X-half.X, c.Y-half.Y), 2*half.X, 2*half.Y)
}

// d = b − a
func apart(d, ha, hb geom.Vec, gap float64) bool {
	return math.Abs(d.X) >= ha.X+hb.X+gap || math.Abs(d.Y) >= ha.Y+hb.Y+gap
}
