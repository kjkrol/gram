package board

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
)

// Entities makes a game's kinds of entities standing on the board, a lever, a trapdoor, a gate:
// each stands still on the cell its row names, in a box of the Shape's Size at the cell's middle.
type Entities[P any] struct {
	brd   *Plugin
	shape Shape
	at    func(row P) cell.ID
}

// NewEntities binds a game's rows of entities standing on the board to it: shape is their body,
// at the cell a row stands one on.
func NewEntities[P any](brd *Plugin, shape Shape, at func(row P) cell.ID) *Entities[P] {
	return &Entities[P]{brd: brd, shape: shape, at: at}
}

// Define registers one kind of entity standing on the board, its entities carrying extra
// besides, in place of what the roster's Standing template gives and of a Z as tall as its shape;
// one standing off the board panics as it is spawned.
func (e *Entities[P]) Define(name string, extra ...comp.Comp) {
	brd := &e.brd.board.read
	own := []comp.Comp{
		comp.Load(func(row P) world.Position {
			c := e.at(row)
			if _, ok := brd.Ordinal(c); !ok {
				panic(fmt.Sprintf("board: a %q stands off the board, on cell %d", name, c))
			}
			return world.Position{AABB: e.shape.Box(brd.CellCenter(c))}
		}),
		comp.Load(func(row P) cell.On { return cell.On{Cell: e.at(row)} }),
	}
	// a Z as tall as its shape: the template gives every kind the same
	if !slices.ContainsFunc(extra, func(c comp.Comp) bool {
		return c.Type() == reflect.TypeFor[world.Z]()
	}) {
		own = append(own, comp.Const(world.Z{Height: e.shape.Height}))
	}
	spec := e.brd.worldPlugin.Roster().Spec(StandingTemplate, append(own, extra...)...)
	kind.Define[P](e.brd.worldPlugin.Kinds(), name, spec)
	if e.brd.cellOf == nil {
		e.brd.cellOf = map[string]func(any) cell.ID{}
	}
	e.brd.cellOf[name] = func(row any) cell.ID { return e.at(row.(P)) }
}

// Named is the kind of entity defined as name.
func (e *Entities[P]) Named(name string) kind.Of[P] {
	return kind.Named[P](e.brd.worldPlugin.Kinds(), name)
}
