package main

import (
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	fuseLength  = 2 * time.Second
	blastRadius = 2.5 * CellSize
	blastKills  = CellSize
)

// the ammo is the bullet plugin's register, no figure: its parts are called from the arena's
// sections, its looks beside the figures'
func (a *arena) ammoEffects() {
	a.world.Effects().Define(FuseEf, effect.Spec{effect.Lasts(fuseLength)})
}

// a round wounds and kills the wounded; a grenade's blast kills within blastKills, wounds beyond
func (a *arena) ammoRules() {
	roles := a.world.Roles()
	wounded := a.world.Effects().Named(WoundedEf)
	mortal := roles.Named(MortalRole)
	roles.Define(RoundRole, act.Branch[collision.Meeting]{
		Name: "shot",
		When: cond.Other.If(mortal),
		Then: act.OneOf(
			act.Branch[any]{When: cond.Other.If(wounded), Then: act.Other.Do(world.Despawn{})},
			act.Other.Put(wounded),
		),
	})
	roles.Define(GrenadeRole, act.Branch[bullet.Blast]{
		Name: "blast",
		When: cond.Other.If(mortal),
		Then: act.OneOf(
			act.Branch[bullet.Blast]{
				When: cond.Other.If(bullet.Within(blastKills)),
				Then: act.Other.Do(world.Despawn{}),
			},
			act.Other.Put(wounded),
		),
	})
}

func (a *arena) defineAmmo() {
	roles := a.world.Roles()
	a.bullet.Shots().Define(RoundKind,
		bullet.Body{Size: 4, Speed: 10 * CellSize, Range: 10 * CellSize},
		ceremony.Plays(roles.Named(RoundRole)))
	fuse := plan.New(FusePlan, plan.Steps(
		plan.Loop[bullet.Flight]{
			Name: "lit",
			When: cond.Self.If(bullet.Lying()),
			Then: plan.Steps(
				plan.Self.Put(a.world.Effects().Named(FuseEf)),
				plan.Self.Wait(fuseLength),
			),
		},
		plan.Loop[any]{
			Name: "burst",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Do(bullet.Burst{Radius: blastRadius}),
		},
	))
	a.bullet.Shots().Define(GrenadeKind,
		bullet.Body{Size: 8, Speed: 5 * CellSize, Range: 9 * CellSize, Gravity: 240, Lands: true},
		ceremony.Plays(roles.Named(GrenadeRole)), fuse)
}

func (a *arena) ammoLook(atlas *world.Atlas) {
	atlas.Add(a.bullet.Shots().Named(RoundKind), 8, render.Arrow(0, 2, roundColor)).
		Turning() // the plugin keeps a shot's heading on its Vel.Dir
	atlas.Add(a.bullet.Shots().Named(GrenadeKind), 8, render.Dot(3, grenadeColor)).
		Under(a.world.Effects().Named(FuseEf), func(dst *render.Canvas, size int) {
			render.Dot(3, grenadeColor)(dst, size)
			render.Diamond(sparkColor)(dst, size)
		})
}
