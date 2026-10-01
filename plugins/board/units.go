package board

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// Units makes a game's unit kinds over the board: from where a row says the unit stands it
// derives its Position and Cell, from the kind's Mover its Layers, and in a world with heights its Z
// from the Shape.
type Units[P any] struct {
	brd   *Plugin
	shape Shape
	at    func(row P) geom.Vec
}

// Shape is a unit's body: the side of its square box and, in a world with heights, how tall it stands.
type Shape struct{ Size, Height float64 }

// NewUnits binds a game's rows to the board: shape is the units' body, at reads a unit's centre
// from its row — a game that thinks in cells hands over their CellCenter.
func NewUnits[P any](brd *Plugin, shape Shape, at func(row P) geom.Vec) *Units[P] {
	if !brd.worldPlugin.HasHeights() && shape.Height != 0 {
		panic("board: units with a Height in a flat world; set world.Config.Heights")
	}
	return &Units[P]{brd: brd, shape: shape, at: at}
}

// Define registers one kind of unit: its name, how it moves (the domains, and in a world with heights
// the Lift it keeps above the ground), its steering profile and whatever else the game gives its
// entities. It is kind.Define with the board's part filled in and the world's roster checked; a
// unit standing off the board panics when spawned.
func (u *Units[P]) Define(name string, mover Mover, steering steering.Steering, extra ...comp.Comp) kind.Of[P] {
	brd := u.brd.Res.Logic.Board
	heights := u.brd.worldPlugin.HasHeights()
	if !heights && mover.Lift != 0 {
		panic(fmt.Sprintf("board: %q has a Lift in a flat world; set world.Config.Heights", name))
	}
	half := u.shape.Size / 2
	own := []comp.Comp{
		comp.Load(func(row P) world.Position {
			c := u.at(row)
			return world.Position{AABB: plane.NewAABB(geom.NewVec(c.X-half, c.Y-half), u.shape.Size, u.shape.Size)}
		}),
		comp.Load(func(row P) At {
			c, ok := brd.CellAt(u.at(row))
			if !ok {
				panic(fmt.Sprintf("board: a %q stands off the board at %v", name, u.at(row)))
			}
			return At{Cell: c}
		}),
		comp.Const(mover),
		comp.Const(world.Layers(mover.Domain)),
		comp.Const(steering),
	}
	if heights {
		own = append(own, comp.Const(world.Z{Height: u.shape.Height})) // Altitude is the board's to write
	}
	spec := u.brd.worldPlugin.Roster().Unit.Spec(append(own, extra...)...)
	return kind.Define[P](u.brd.worldPlugin.Kinds(), name, spec)
}
