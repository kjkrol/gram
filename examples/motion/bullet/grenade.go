package main

import (
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	fuseLength  = 2 * time.Second
	blastRadius = 2.5 * CellSize
	blastKills  = CellSize
)

// grenade lies lit for its fuse and bursts: kills within blastKills, wounds beyond
type grenade struct{}

var (
	_ bullet.Figure   = grenade{}
	_ bullet.Stateful = grenade{}
	_ bullet.Ruled    = grenade{}
)

func (grenade) Effects(p *bullet.Plugin) {
	p.World().Effects().Define(FuseEf, effect.Spec{effect.Lasts(fuseLength)})
}

func (grenade) Rules(p *bullet.Plugin) {
	roles, wounded := p.World().Roles(), p.World().Effects().Named(WoundedEf)
	roles.Define(GrenadeRole, act.Branch[bullet.Blast]{
		Name: "blast",
		When: cond.Other.If(roles.Named(MortalRole)),
		Then: act.OneOf(
			act.Branch[bullet.Blast]{
				When: cond.Other.If(bullet.Within(blastKills)),
				Then: act.Other.Do(world.Despawn{}),
			},
			act.Other.Put(wounded),
		),
	})
}

func (grenade) Define(p *bullet.Plugin) {
	fuse := plan.New(FusePlan, plan.Steps(
		plan.Loop[bullet.Flight]{
			Name: "lit",
			When: cond.Self.If(bullet.Lying()),
			Then: plan.Steps(
				plan.Self.Put(p.World().Effects().Named(FuseEf)),
				plan.Self.Wait(fuseLength),
			),
		},
		plan.Loop[any]{
			Name: "burst",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Do(bullet.Burst{Radius: blastRadius}),
		},
	))
	p.Shots().Define(GrenadeKind,
		bullet.Body{Size: 8, Speed: 5 * CellSize, Range: 9 * CellSize, Gravity: 240, Lands: true},
		ceremony.Plays(p.World().Roles().Named(GrenadeRole)), fuse)
}

func (grenade) Look(a *world.Atlas, p *bullet.Plugin) {
	a.Add(p.Shots().Named(GrenadeKind), 8, render.Dot(3, grenadeColor)).
		Under(p.World().Effects().Named(FuseEf), lit)
}

func lit(dst *render.Canvas, size int) {
	render.Dot(3, grenadeColor)(dst, size)
	render.Diamond(sparkColor)(dst, size)
}
