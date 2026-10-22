package main

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
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

type guardRow struct {
	Start cell.ID    `yaml:"start"`
	Round [2]cell.ID `yaml:"round"`
}

// guard patrols its round, steps aside when stuck, rests where it arrives and runs to the gate
// while the alarm is on
type guard struct{}

var _ board.Figure = guard{}

func (guard) Define(b *board.Plugin) {
	w := b.Read(0).World()
	duty := plan.New(GuardPlan, plan.OneOf(
		plan.Loop[any]{
			Name:    "to the gate",
			When:    cond.On(w).If(w.Effects().Named(AlarmEf)),
			Then:    plan.Self.Do(navigation.MoveTo{To: ceremony.Named(GateName)}),
			Until:   cond.Self.If(navigation.Arrived{}),
			Timeout: 20 * time.Second,
		},
		plan.Loop[navigation.Blocked]{
			Name: "blocked",
			When: cond.Self.If(navigation.Stuck()),
			Then: plan.Self.Do(navigation.StepAside{}),
		},
		plan.Loop[navigation.Arrived]{
			Name: "rest",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(2 * time.Second),
		},
		plan.Loop[any]{
			Name:  "patrol",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "idle", When: cond.Self.If(cond.Always), Then: plan.Self.Idle()},
	))
	guards := board.NewUnits[guardRow](b, board.Shape{Size: unitSize},
		func(g guardRow) geom.Vec { return b.Read(0).Board().CellCenter(g.Start) })
	guards.Define(GuardKind, unit.Mover{Domain: cell.Land}, profile, duty,
		comp.Load(func(g guardRow) navigation.Round {
			return navigation.NewRound(g.Round[:]...)
		}))
}

func (guard) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[guardRow](w.Kinds(), GuardKind), unitSize, render.Solid(guardColor))
}
