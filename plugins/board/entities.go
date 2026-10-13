package board

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
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
// at the cell a row stands one on. A Height in a flat world panics.
func NewEntities[P any](brd *Plugin, shape Shape, at func(row P) cell.ID) *Entities[P] {
	if !brd.worldPlugin.HasHeights() && shape.Height != 0 {
		panic("board: entities with a Height in a flat world; set world.Config.Heights")
	}
	return &Entities[P]{brd: brd, shape: shape, at: at}
}

// Define registers one kind of entity standing on the board, its entities carrying extra
// besides; one standing off the board panics as it is spawned.
func (e *Entities[P]) Define(name string, extra ...comp.Comp) {
	brd := e.brd.Board()
	size := e.shape.Size
	own := []comp.Comp{
		comp.Load(func(row P) world.Position {
			c := e.at(row)
			if _, ok := brd.Ordinal(c); !ok {
				panic(fmt.Sprintf("board: a %q stands off the board, on cell %d", name, c))
			}
			m := brd.CellCenter(c)
			top := geom.NewVec(m.X-size/2, m.Y-size/2)
			return world.Position{AABB: plane.NewAABB(top, size, size)}
		}),
		comp.Load(func(row P) cell.On { return cell.On{Cell: e.at(row)} }),
	}
	if !slices.ContainsFunc(extra, func(c comp.Comp) bool {
		return c.Type() == reflect.TypeFor[world.Velocity]()
	}) {
		own = append(own, comp.Const(world.Velocity{}))
	}
	if !slices.ContainsFunc(extra, func(c comp.Comp) bool {
		return c.Type() == reflect.TypeFor[world.Lie]()
	}) {
		own = append(own, comp.Const(world.Lie{}))
	}
	if e.brd.worldPlugin.HasHeights() {
		own = append(own, comp.Const(world.Z{Height: e.shape.Height}))
	}
	kind.Define[P](e.brd.worldPlugin.Kinds(), name, append(own, extra...))
	if e.brd.cellOf == nil {
		e.brd.cellOf = map[string]func(any) cell.ID{}
	}
	e.brd.cellOf[name] = func(row any) cell.ID { return e.at(row.(P)) }
}

// Named is the kind of entity defined as name.
func (e *Entities[P]) Named(name string) kind.Of[P] {
	return kind.Named[P](e.brd.worldPlugin.Kinds(), name)
}
