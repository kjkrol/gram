package board

import (
	"fmt"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

// Units makes a game's unit kinds over the board, deriving Position, At, Layers and Z from where
// a row says the unit stands, its Mover and the Shape.
type Units[P any] struct {
	brd   *Plugin
	shape Shape
	at    func(row P) geom.Vec
}

// Shape is a unit's body: the side of its box, Width across it where the box is no square (0:
// Size, square), and how tall it stands.
type Shape struct{ Size, Width, Height float32 }

// Box is the shape's box centred on c: Width across, Size down the screen.
func (s Shape) Box(c geom.Vec) plane.AABB {
	w := s.Width
	if w == 0 {
		w = s.Size
	}
	return plane.NewAABB(geom.NewVec(c.X-w/2, c.Y-s.Size/2), w, s.Size)
}

// NewUnits binds a game's rows to the board: shape is the units' body, at reads a unit's centre
// from its row. A unit bigger than a cell is planned where every cell its footprint takes takes it
// (unit.Footprint); a footprint past the board's (WithFootprint), or a board without motion and
// a mobility (Plugin.WithMotion), panics.
func NewUnits[P any](brd *Plugin, shape Shape, at func(row P) geom.Vec) *Units[P] {
	if brd.motion == nil || brd.mobility == nil {
		panic("board: units move by motion as a mobility paces them, and the board was given " +
			"none (WithMotion)")
	}
	box := shape.Box(geom.Vec{})
	if cells := unit.Footprint(&brd.board.read, box.Size); cells > brd.footprint {
		panic(fmt.Sprintf("board: units %v across and %v down take a footprint of %d cells a "+
			"side, past the board's %d (WithFootprint): past a cell a footprint holds its box "+
			"with room to spare, slack for collision", box.Size.X, box.Size.Y, cells,
			brd.footprint))
	}
	return &Units[P]{brd: brd, shape: shape, at: at}
}

// DefaultFootprint is the most cells a side a unit's footprint takes on a board: a box past a cell
// narrower than three cells.
const DefaultFootprint = 3

// WithFootprint has the board take units whose footprint takes up to cells cells a side
// (unit.Footprint; DefaultFootprint without): the wider, the wider the planner's work round every
// cell changed. Call before NewUnits.
func (p *Plugin) WithFootprint(cells int) *Plugin {
	if cells < 1 {
		panic(fmt.Sprintf("board: a footprint of %d cells a side; one is the least", cells))
	}
	p.footprint = cells
	return p
}

// Define registers one kind of unit: how it moves, its steering profile and whatever else the
// game gives its entities. A unit standing off the board panics when spawned.
func (u *Units[P]) Define(name string, mover unit.Mover, steering steering.Steering, extra ...comp.Comp) {
	k := UnitKind{Name: name, Mover: mover, Shape: u.shape}
	brd := &u.brd.board.read
	own := []comp.Comp{
		comp.Load(func(row P) world.Position {
			return world.Position{AABB: u.shape.Box(u.at(row))}
		}),
		comp.Load(func(row P) unit.At {
			c, ok := brd.CellAt(unit.LaidFrom(brd, k.Shape.Box(geom.Vec{}).Size, u.at(row)))
			if !ok {
				panic(fmt.Sprintf("board: a %q stands off the board at %v", name, u.at(row)))
			}
			return unit.At{Cell: c}
		}),
		comp.Const(mover),
		comp.Const(world.Layers(mover.Domain)),
		comp.Const(steering),
		comp.Const(k.Z()), // Altitude is the board's to write
	}
	spec := u.brd.worldPlugin.Roster().Spec(motion.UnitTemplate, append(own, extra...)...)
	kind.Define[P](u.brd.worldPlugin.Kinds(), name, spec)
	u.brd.unitKinds = append(u.brd.unitKinds, k)
}

// Named is the kind of unit defined as name.
func (u *Units[P]) Named(name string) kind.Of[P] {
	return kind.Named[P](u.brd.worldPlugin.Kinds(), name)
}

// UnitKind is a kind of unit defined over the board: its name, how it moves and its body.
type UnitKind struct {
	Name  string
	Mover unit.Mover
	Shape Shape
}

// Z is the place in height the kind's units carry: the Shape's Height, their Altitude the
// board's to write.
func (k UnitKind) Z() world.Z { return world.Z{Height: k.Shape.Height} }

// UnitKinds are the kinds of unit defined over the board so far, in the order defined.
func (p *Plugin) UnitKinds() []UnitKind { return slices.Clone(p.unitKinds) }
