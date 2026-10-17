package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const hawkPx = 22

var hawkSpeed = walkerSpeed * 1.5

// hawk flies over everything up to the cloud base, looking down, sent to its target from the map.
type hawk struct{}

var _ board.Figure = hawk{}

func (hawk) Define(b *board.Plugin) {
	units := board.NewUnits[unitRow](b,
		board.Shape{Size: scale.Figure(0.6), Height: figure},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	flyer := unit.Mover{
		Domain:    cell.Air,
		Lift:      scale.Units(300),
		Clearance: scale.Units(20),
		Ceiling:   scale.Units(air.CloudBase - 100),
	}
	profile := steering.Steering{
		MaxSpeed: hawkSpeed,
		Sprint:   sprint,
		Accel:    walkerSpeed * 2,
		Brake:    walkerSpeed * 4,
		V0:       walkerSpeed / 2,
		TurnRate: 0.1,
	}
	units.Define(HawkKind, flyer, profile, append(looking(), ordered())...)
}

func (hawk) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), HawkKind), hawkPx, render.Diamond(hawkColor))
}
