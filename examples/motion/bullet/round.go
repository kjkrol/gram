package main

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// round wounds whom it strikes and kills the wounded
type round struct{}

var (
	_ bullet.Figure = round{}
	_ bullet.Ruled  = round{}
)

func (round) Rules(p *bullet.Plugin) {
	roles, wounded := p.World().Roles(), p.World().Effects().Named(WoundedEf)
	roles.Define(RoundRole, act.Branch[collision.Meeting]{
		Name: "shot",
		When: cond.Other.If(roles.Named(MortalRole)),
		Then: act.OneOf(
			act.Branch[any]{When: cond.Other.If(wounded), Then: act.Other.Do(world.Despawn{})},
			act.Other.Put(wounded),
		),
	})
}

func (round) Define(p *bullet.Plugin) {
	p.Shots().Define(RoundKind,
		bullet.Body{Size: 4, Speed: 10 * CellSize, Range: 10 * CellSize},
		ceremony.Plays(p.World().Roles().Named(RoundRole)))
}

func (round) Look(a *world.Atlas, p *bullet.Plugin) {
	a.Add(p.Shots().Named(RoundKind), 8, render.Arrow(0, 2, roundColor)).
		Turning() // the plugin keeps a shot's heading on its Vel.Dir
}
