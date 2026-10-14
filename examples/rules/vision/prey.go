package main

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

// prey roams and bounces; skittish while the flight is on, it turns away from the hunter in
// sight and from whoever comes at it
type prey struct{}

var (
	_ world.Figure = prey{}
	_ world.Ruled  = prey{}
)

func (prey) Rules(w *world.Plugin) {
	roles, fleeing := w.Roles(), w.Effects().Named(FleeingEf)
	roles.Define(SkittishRole,
		act.Branch[vision.Sighting]{
			Name: "flee the hunter",
			When: cond.Other.If(roles.Named(PredatorRole)).
				And(cond.On(w).If(fleeing)),
			Then: act.Self.Do(steering.Away{}),
		},
		act.Branch[vision.Sighting]{
			Name: "give way",
			When: cond.On(w).If(fleeing).
				And(cond.Other.If(vision.Closing())),
			Then: act.Self.Do(steering.Away{}),
		},
	)
}

func (prey) Define(w *world.Plugin) {
	roles := w.Roles()
	kind.Define[body](w.Kinds(), PreyKind, append(sees(),
		comp.Const(steering.Steering{Reflex: 3, TurnRate: 0.12}),
		ceremony.Plays(roles.Named(SkittishRole), roles.Named(PreyRole)),
		comp.Const(collision.Physics{Restitution: 1}),
	))
}

func (prey) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[body](w.Kinds(), PreyKind), RectSize, render.Solid(preyColor))
}
