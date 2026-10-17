package main

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	heroWidth  = 16
	heroHeight = 32

	heroSpeed = CellSize * 2
	reachFor  = time.Second // a press waits for a swinging lever this long
)

type unitRow struct {
	Start cell.ID `yaml:"start"`
}

// hero is the one the player walks, a rectangle; turned to a lever he touches, he pulls it.
type hero struct{}

var (
	_ board.Figure   = hero{}
	_ board.Stateful = hero{}
	_ board.Ruled    = hero{}
)

func (hero) Effects(b *board.Plugin) {
	fx := b.World().Effects()
	fx.Define(ReachingEf, effect.Spec{effect.Lasts(reachFor)})
	fx.Define(PullingEf, effect.Spec{effect.Lasts(leverSwing)})
	fx.Define(InReachEf, effect.Spec{})
}

// handy, he uses the lever he touches facing it; Mortal is everyone's, the arena's
func (hero) Rules(b *board.Plugin) {
	roles, fx := b.World().Roles(), b.World().Effects()
	atHand := cond.Other.If(roles.Named(LeverRole)).And(cond.Self.If(collision.Facing(math.Pi / 4)))
	// a lever swinging is not pulled again: the hands stay on it as long as the swing
	leverStill := cond.Other.If(cond.Not(fx.Named(LeverPullingEf))).
		And(cond.Other.If(cond.Not(fx.Named(LeverPushingEf))))
	roles.Define(HandyRole,
		act.Branch[collision.Meeting]{
			Name: "a lever at hand",
			When: atHand,
			Then: act.Self.Hold(fx.Named(InReachEf)),
		},
		act.Branch[collision.Meeting]{
			Name: "pull the lever at hand",
			When: cond.Self.If(fx.Named(ReachingEf)).And(atHand).And(leverStill).
				And(cond.Other.If(cond.Not(fx.Named(LeverPulledEf)))),
			Then: act.Steps(
				act.Self.Take(fx.Named(ReachingEf)),
				act.Self.Put(fx.Named(PullingEf)),
				act.Other.Put(fx.Named(LeverPullingEf)),
				act.Other.Do(ceremony.Trigger{}),
			),
		},
		act.Branch[collision.Meeting]{
			Name: "push the lever at hand",
			When: cond.Self.If(fx.Named(ReachingEf)).And(atHand).And(leverStill).
				And(cond.Other.If(fx.Named(LeverPulledEf))),
			Then: act.Steps(
				act.Self.Take(fx.Named(ReachingEf)),
				act.Self.Put(fx.Named(PullingEf)),
				act.Other.Take(fx.Named(LeverPulledEf)),
				act.Other.Put(fx.Named(LeverPushingEf)),
				act.Other.Do(ceremony.Trigger{}),
			),
		},
	)
}

func (hero) Define(b *board.Plugin) {
	roles := b.World().Roles()
	units := board.NewUnits[unitRow](b,
		board.Shape{Size: heroHeight, Width: heroWidth, Height: heroHeight},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: heroSpeed,
		Accel:    heroSpeed * 4, // full speed in a quarter of a second
		Brake:    heroSpeed * 6,
	}
	units.Define(HeroKind, unit.Mover{Domain: cell.Land}, profile,
		comp.Const(world.Velocity{Dir: geom.NewVec(0, 1)}), // facing the player
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
		ceremony.Plays(roles.Named(HandyRole), roles.Named(MortalRole)))
}

func (hero) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), HeroKind), heroHeight, render.Solid(heroColor))
}
