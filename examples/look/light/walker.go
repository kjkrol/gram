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
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	walkerPx     = 20
	walkerHeight = 28
	walkerSpeed  = CellSize * 2
)

type walkerRow struct {
	Start cell.ID    `yaml:"start"`
	Round [4]cell.ID `yaml:"round"`
}

// walker patrols its round from the map, under the tower, over the lava and past the lantern,
// casting the soft patch every entity casts by default.
type walker struct{}

var _ board.Figure = walker{}

func (walker) Define(b *board.Plugin) {
	units := board.NewUnits[walkerRow](b, board.Shape{Size: walkerPx, Height: walkerHeight},
		func(u walkerRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: walkerSpeed,
		Accel:    walkerSpeed * 2,
		Brake:    walkerSpeed * 4,
		V0:       walkerSpeed / 2,
		TurnRate: 0.15,
	}
	patrol := plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(time.Second)},
	))
	round := comp.Load(func(u walkerRow) navigation.Round {
		return navigation.NewRound(u.Round[:]...)
	})
	units.Define(WalkerKind, unit.Mover{Domain: cell.Land}, profile, round, patrol)
}

func (walker) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[walkerRow](w.Kinds(), WalkerKind), walkerPx, render.Diamond(walkerColor))
}
