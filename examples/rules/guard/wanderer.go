package main

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
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

type wandererRow struct {
	Start cell.ID `yaml:"start"`
	To    cell.ID `yaml:"to"`
}

// wanderer walks to and fro across the yard, in the guards' way
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
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(3 * time.Second)},
	))
	wanderers := board.NewUnits[wandererRow](b, board.Shape{Size: unitSize},
		func(w wandererRow) geom.Vec { return b.Board().CellCenter(w.Start) })
	wanderers.Define(WandererKind, unit.Mover{Domain: cell.Land}, profile, patrol,
		comp.Load(func(w wandererRow) navigation.Round {
			return navigation.NewRound(w.To, w.Start)
		}))
}

func (wanderer) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[wandererRow](w.Kinds(), WandererKind), unitSize,
		render.Solid(wandererColor))
}
