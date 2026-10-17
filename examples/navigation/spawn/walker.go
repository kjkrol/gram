package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	walkerPx    = 22
	walkerSpeed = CellSize * 2
)

type walkerRow struct {
	Start cell.ID `yaml:"start"`
}

// walker is a unit on foot, laid by the map or spawned under the cursor.
type walker struct{}

var _ board.Figure = walker{}

func (walker) Define(b *board.Plugin) {
	units := board.NewUnits[walkerRow](b, board.Shape{Size: walkerPx},
		func(w walkerRow) geom.Vec { return b.Board().CellCenter(w.Start) })
	units.Define(WalkerKind, unit.Mover{Domain: cell.Land}, steering.Steering{
		MaxSpeed: walkerSpeed,
		Accel:    walkerSpeed * 2,
		Brake:    walkerSpeed * 4,
		V0:       walkerSpeed / 2,
		TurnRate: 0.15,
	})
}

func (walker) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[walkerRow](w.Kinds(), WalkerKind), walkerPx, render.Solid(walkerColor))
}
