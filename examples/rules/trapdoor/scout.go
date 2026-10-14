package main

import (
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

// scout is the player's unit, sent about by a right click and hastened for a while on J
type scout struct{}

var (
	_ board.Figure   = scout{}
	_ board.Stateful = scout{}
)

func (scout) Effects(b *board.Plugin) {
	b.World().Effects().Define(HasteEf, effect.Spec{
		effect.Lasts(2 * time.Second),
		effect.Alter(func(st *steering.Steering) {
			st.MaxSpeed *= 2
			st.Accel *= 2
		}),
	})
}

func (scout) Define(b *board.Plugin) {
	units(b).Define(ScoutKind, unit.Mover{Domain: cell.Land}, profile,
		ceremony.Plays(b.World().Roles().Named(MortalRole)))
}

func (scout) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), ScoutKind), unitSize, render.Solid(scoutColor)).
		Under(w.Effects().Named(HasteEf), render.Solid(hasteColor))
}
