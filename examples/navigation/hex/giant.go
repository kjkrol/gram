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
	giantPx     = 60 // past a hex: a footprint of three hexes a side, its hex and the ring round it
	giantSpeed  = HexStep
	giantWeight = 3000 // kg
)

// giant is a walker too big for every way a hex wide, the wall's gap and the bridges: its routes go
// by the hole in the wall and the ford, both three hexes wide.
type giant struct{}

var _ board.Figure = giant{}

func (giant) Define(b *board.Plugin) {
	profile := steering.Steering{
		MaxSpeed: giantSpeed,
		Accel:    giantSpeed * 2,
		Brake:    giantSpeed * 4,
		V0:       giantSpeed / 2,
		TurnRate: 0.08,
	}
	units := board.NewUnits[unitRow](b, board.Shape{Size: giantPx, Height: 4},
		func(u unitRow) geom.Vec {
			return unit.Place(b.Read(0).Board(), geom.NewVec(giantPx, giantPx), u.Start)
		})
	units.Define(GiantKind, unit.Mover{Domain: cell.Land, Weight: giantWeight}, profile,
		ordered())
}

func (giant) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), GiantKind), giantPx, render.Solid(giantColor)).
		Styled(selection.Highlight{Line: giantColor, Width: 2}).
		Styled(navigation.RouteStyle{Line: giantColor, Goal: giantColor, Width: 1.5})
}
