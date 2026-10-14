package main

import (
	"github.com/kjkrol/gram/plugins/selection"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

const (
	walkerPx    = 22
	walkerSpeed = CellSize * 2
)

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

// walker is a unit on foot, mortal, sent to its target from the map; one kind a colour.
type walker struct {
	kind  string
	color color.RGBA
}

var _ board.Figure = walker{}

func (wk walker) Define(b *board.Plugin) {
	units := board.NewUnits[unitRow](b, board.Shape{Size: walkerPx},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: walkerSpeed,
		Accel:    walkerSpeed * 2,
		Brake:    walkerSpeed * 4,
		V0:       walkerSpeed / 2,
		TurnRate: 0.15,
	}
	units.Define(wk.kind, unit.Mover{Domain: cell.Land}, profile,
		comp.Load(func(u unitRow) navigation.MoveOrder {
			return navigation.MoveOrder{Target: u.Target}
		}),
		ceremony.Plays(b.World().Roles().Named(MortalRole)))
}

// each side outlined and routed in its own colour
func (wk walker) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), wk.kind), walkerPx, render.Solid(wk.color)).
		Styled(selection.Highlight{Line: wk.color, Width: 2}).
		Styled(navigation.RouteStyle{Line: wk.color, Goal: wk.color, Width: 1.5})
}
