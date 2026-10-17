package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	walkerPx    = 24
	walkerSpeed = CellSize * 2
)

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

// walker sets out for its target from the map
type walker struct{}

var _ board.Figure = walker{}

func (walker) Define(b *board.Plugin) {
	units := board.NewUnits[unitRow](b, board.Shape{Size: walkerPx},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: walkerSpeed,
		Accel:    walkerSpeed * 2,
		Brake:    walkerSpeed * 4,
		V0:       walkerSpeed / 2,
		TurnRate: 0.15,
	}
	order := comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.MoveOrder{Target: u.Target}
	})
	units.Define(UnitKind, unit.Mover{Domain: cell.Land}, profile, order)
}

func (walker) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), UnitKind), walkerPx, render.Diamond(unitColor))
}
