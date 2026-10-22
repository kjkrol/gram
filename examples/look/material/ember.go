package main

import (
	"embed"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
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
	emberPx    = 28
	emberSpeed = CellSize * 3
)

//go:embed ember.wgsl
var emberFS embed.FS

// registered before the composer's shader compiles
var emberMaterial = render.RegisterMaterials(render.Files(emberFS, "ember.wgsl"), nil, "Ember")[0]

type unitRow struct {
	Start cell.ID    `yaml:"start"`
	Round [4]cell.ID `yaml:"round"`
}

// ember is drawn by a material out of its own state, breathing standing and streaming driven;
// doused it is soot for a while. It patrols its round from the map.
type ember struct{}

var (
	_ board.Figure   = ember{}
	_ board.Stateful = ember{}
)

func (ember) Effects(b *board.Plugin) {
	b.Read(0).World().Write().Effects().Define(DousedEf, effect.Spec{
		effect.Described("Doused: the flame put out — soot until it rekindles."),
		effect.Lasts(3 * time.Second),
	})
}

func (ember) Define(b *board.Plugin) {
	units := board.NewUnits[unitRow](b, board.Shape{Size: emberPx},
		func(u unitRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: emberSpeed,
		Accel:    emberSpeed * 2,
		Brake:    emberSpeed * 4,
		V0:       emberSpeed / 2,
		TurnRate: 0.1,
	}
	patrol := plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(2 * time.Second)},
	))
	round := comp.Load(func(u unitRow) navigation.Round {
		return navigation.NewRound(u.Round[:]...)
	})
	units.Define(EmberKind, unit.Mover{Domain: cell.Land}, profile, round, patrol)
}

func (ember) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), EmberKind), emberPx, emberMaterial).
		Under(w.Write().Effects().Named(DousedEf), render.Dot(5, sootColor))
}
