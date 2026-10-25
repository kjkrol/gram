package board

import (
	"fmt"

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
// from its row. A unit is no bigger than a cell's side, so it takes any step a deck does not
// narrow; a bigger shape, or a board without motion (Plugin.WithMotion), panics.
func NewUnits[P any](brd *Plugin, shape Shape, at func(row P) geom.Vec) *Units[P] {
	if brd.motion == nil {
		panic("board: units move by motion, and the board was given none (WithMotion)")
	}
	if box, side := shape.Box(geom.Vec{}), brd.board.read.CellSpan(); max(box.Size.X,
		box.Size.Y) > side {
		panic(fmt.Sprintf("board: units %v across and %v down, bigger than a cell's side, %v",
			box.Size.X, box.Size.Y, side))
	}
	return &Units[P]{brd: brd, shape: shape, at: at}
}

// Define registers one kind of unit: how it moves, its steering profile and whatever else the
// game gives its entities. A unit standing off the board panics when spawned.
func (u *Units[P]) Define(name string, mover unit.Mover, steering steering.Steering, extra ...comp.Comp) {
	brd := &u.brd.board.read
	own := []comp.Comp{
		comp.Load(func(row P) world.Position {
			return world.Position{AABB: u.shape.Box(u.at(row))}
		}),
		comp.Load(func(row P) unit.At {
			c, ok := brd.CellAt(u.at(row))
			if !ok {
				panic(fmt.Sprintf("board: a %q stands off the board at %v", name, u.at(row)))
			}
			return unit.At{Cell: c}
		}),
		comp.Const(mover),
		comp.Const(world.Layers(mover.Domain)),
		comp.Const(steering),
		comp.Const(world.Z{Height: u.shape.Height}), // Altitude is the board's to write
	}
	spec := u.brd.worldPlugin.Roster().Spec(motion.UnitTemplate, append(own, extra...)...)
	kind.Define[P](u.brd.worldPlugin.Kinds(), name, spec)
}

// Named is the kind of unit defined as name.
func (u *Units[P]) Named(name string) kind.Of[P] {
	return kind.Named[P](u.brd.worldPlugin.Kinds(), name)
}
