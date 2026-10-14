package main

import (
	"math"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

// hunter chases the prey in sight, catches what it strikes, and seeing nobody turns a quarter
// either way, once a second
type hunter struct{}

var (
	_ world.Figure   = hunter{}
	_ world.Stateful = hunter{}
	_ world.Ruled    = hunter{}
)

func (hunter) Effects(w *world.Plugin) {
	w.Effects().Define(TurnedEf, effect.Spec{effect.Lasts(time.Second)})
}

func (hunter) Rules(w *world.Plugin) {
	roles, turned := w.Roles(), w.Effects().Named(TurnedEf)
	prey := roles.Named(PreyRole)
	left, right := steering.Turn{Angle: math.Pi / 2}, steering.Turn{Angle: -math.Pi / 2}
	roles.Define(PredatorRole,
		act.Branch[vision.Sighting]{
			Name: "chase",
			When: cond.Other.If(prey),
			Then: act.Self.Do(steering.Toward{}),
		},
		act.Branch[vision.Sighting]{
			Name: "search",
			When: cond.Self.If(vision.SeesNobody()),
			Then: act.Branch[any]{
				When: cond.Self.If(cond.Not(turned)),
				Then: act.Steps(
					act.Self.Put(turned),
					act.OneOf(
						act.Branch[any]{
							When: cond.Self.If(cond.Chance(0.5)),
							Then: act.Self.Do(left),
						},
						act.Self.Do(right),
					),
				),
			},
		},
		act.Branch[collision.Meeting]{
			Name: "caught",
			When: cond.Other.If(prey),
			Then: act.Other.Do(world.Despawn{}),
		},
	)
}

func (hunter) Define(w *world.Plugin) {
	kind.Define[body](w.Kinds(), HunterKind, append(sees(),
		comp.Const(steering.Steering{Reflex: 1, TurnRate: 0.30}),
		ceremony.Plays(w.Roles().Named(PredatorRole)),
	))
}

func (hunter) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[body](w.Kinds(), HunterKind), RectSize, render.Solid(hunterColor))
}
