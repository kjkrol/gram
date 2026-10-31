package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	wagonPx     = 28 // wider than the bridge's lane between its railings
	wagonSpeed  = CellSize * 2
	wagonWeight = 1500 // kg: more than the rope bridge bears
)

// wagon is too wide for the bridge and too heavy for the rope bridge: its routes go round by the
// ford.
type wagon struct{}

var _ board.Figure = wagon{}

func (wagon) Define(b *board.Plugin) {
	profile := steering.Steering{
		MaxSpeed: wagonSpeed,
		Accel:    wagonSpeed * 2,
		Brake:    wagonSpeed * 4,
		V0:       wagonSpeed / 2,
		TurnRate: 0.1,
	}
	units := board.NewUnits[unitRow](b, board.Shape{Size: wagonPx, Height: 2},
		func(u unitRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) })
	units.Define(WagonKind, unit.Mover{Domain: cell.Land, Weight: wagonWeight}, profile,
		ordered())
}

func (wagon) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), WagonKind), wagonPx, render.Solid(wagonColor)).
		Styled(selection.Highlight{Line: wagonColor, Width: 2}).
		Styled(navigation.RouteStyle{Line: wagonColor, Goal: wagonColor, Width: 1.5})
}
