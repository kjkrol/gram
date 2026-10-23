package main

import (
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// wanderer walks between its two cells from the map, a target for the soldier.
type wanderer struct{}

var _ board.Figure = wanderer{}

func (wanderer) Define(b *board.Plugin) {
	patrol := plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{
			Name: "rest",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(time.Second),
		},
	))
	round := comp.Load(func(u unitRow) navigation.Round {
		return navigation.NewRound(u.To, u.Start)
	})
	units(b).Define(WandererKind, unit.Mover{Domain: cell.Land}, walking(),
		ceremony.Plays(b.Read(0).World().Roles().Named(MortalRole)), round, patrol)
}

func (wanderer) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), WandererKind), unitSize, render.Solid(wandererColor)).
		Under(w.Write().Effects().Named(WoundedEf), render.Solid(paleColor))
}
