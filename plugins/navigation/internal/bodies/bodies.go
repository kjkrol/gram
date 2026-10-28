package bodies

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Body is a unit as the others meet it this tick: where it stands, how it moves and where it
// heads.
type Body struct {
	ID        uid.UID64
	At        geom.Vec // the middle of its box
	Half      geom.Vec
	Vel       geom.Vec // world units a second
	Domain    cell.Domain
	Moving    bool
	GivingWay bool
	Owners    tag.Tags[owner.Family]
	// the order's MoveTo, else the last it came to the end of; zero, none
	Group     uint32
	Goal      cell.ID
	Spot      geom.Vec
	LastGoal  bool
	WaitedOut bool
	Cornered  bool
	Minded    bool
	Cell      cell.ID
	Facing    geom.Vec // the way it faces, standing too
	Z         world.Z
	Lift      float32
	Share     float32 // of its top speed, what the ground leaves it this tick: 1 without a Pace
	MaxSpeed  float32 // world units a second
	Trailing  bool    // follows someone, for a while yet
	TrailPace float32 // while Trailing, the ground speed whom it follows lawfully makes
	// the point its order was given for, round which its group stands; zero with none
	Point geom.Vec
}

// Member is b as a member of the tick, standing where it is.
func (b Body) Member() Member {
	return Member{ID: b.ID, Cell: b.Cell, From: b.Cell, Domain: b.Domain, Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(b.At.X-b.Half.X, b.At.Y-b.Half.Y), 2*b.Half.X, 2*b.Half.Y)},
		Vel: b.Vel, Facing: b.Facing, Z: b.Z, Lift: b.Lift, Owners: b.Owners,
		Share: b.Share, MaxSpeed: b.MaxSpeed}
}

// Alive says b is under way: it moves, or trails someone itself.
func (b Body) Alive() bool { return b.Trailing || b.Vel != (geom.Vec{}) }

// Index is the bodies of a tick by the cells their boxes touch.
type Index struct {
	grid    grid.Grid
	Bodies  []Body
	byID    map[uid.UID64]int
	boxes   [][]int32 // per cell ordinal, the bodies whose box touches the cell
	touched []int
	Largest float32
	adding  int32
	Own     Nearby
	add     func(cell.ID) // the method value, bound once
}

// Nearby gathers the bodies of an index near a box in scratch of its own, for one caller at a
// time.
type Nearby struct {
	x     *Index
	found []int32
	seen  []uint32
	stamp uint32
	find  func(cell.ID) // the method value, bound once
}

// NewIndex is an index over g, empty.
func NewIndex(g grid.Grid) Index { return Index{grid: g} }

// Build indexes bodies, the tick's.
func (x *Index) Build(bodies []Body) {
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
	x.Bodies, x.Largest = bodies, 0
	for i, b := range x.Bodies {
		x.byID[b.ID] = i
		x.Largest = max(x.Largest, b.Half.X, b.Half.Y)
		x.adding = int32(i)
		x.grid.CellsUnder(BoxAt(b.At, b.Half), x.add)
	}
}

func (x *Index) addCell(c cell.ID) {
	if o, ok := x.grid.Ordinal(c); ok {
		if len(x.boxes[o]) == 0 {
			x.touched = append(x.touched, o)
		}
		x.boxes[o] = append(x.boxes[o], x.adding)
	}
}

// Of is the body of id; false for none.
func (x *Index) Of(id uid.UID64) (Body, bool) {
	i, ok := x.byID[id]
	if !ok {
		return Body{}, false
	}
	return x.Bodies[i], true
}

// Near is the bodies whose box touches a cell box touches, each once, by their index in bodies;
// valid till the next call.
func (x *Index) Near(box geom.AABB) []int32 { return x.Own.Near(x, box) }

// Near is x's Near in n's scratch.
func (n *Nearby) Near(x *Index, box geom.AABB) []int32 {
	n.x, n.found = x, n.found[:0]
	if len(x.boxes) == 0 {
		return n.found
	}
	if len(n.seen) < len(x.Bodies) {
		n.seen = append(n.seen, make([]uint32, len(x.Bodies)-len(n.seen))...)
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

func (n *Nearby) findCell(c cell.ID) {
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

// BoxAt is the box of half sizes half round c.
func BoxAt(c, half geom.Vec) geom.AABB {
	return geom.NewAABBAt(geom.NewVec(c.X-half.X, c.Y-half.Y), 2*half.X, 2*half.Y)
}

// Apart reports whether two boxes of half sizes ha and hb, the second d = b − a off the first,
// stand gap or more apart on either axis.
func Apart(d, ha, hb geom.Vec, gap float32) bool {
	return f32.Abs(d.X) >= ha.X+hb.X+gap || f32.Abs(d.Y) >= ha.Y+hb.Y+gap
}
