package main

import (
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

// hawk flies 40 up on the Air plane: it looks over what stops a scout's eye
type hawk struct{}

var _ board.Figure = hawk{}

func (hawk) Define(b *board.Plugin) {
	profile := steering.Steering{
		MaxSpeed: scoutSpeed * 1.5,
		Accel:    scoutSpeed * 2,
		Brake:    scoutSpeed * 4,
		V0:       scoutSpeed / 2,
		TurnRate: 0.1,
	}
	units(b).Define(HawkKind, unit.Mover{Domain: cell.Air, Lift: 40}, profile,
		ordered(), sight(), eye(1))
}

func (hawk) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), HawkKind), scoutPx, render.Diamond(hawkColor)).
		Styled(vision.Cone{Color: hawkColor})
}
