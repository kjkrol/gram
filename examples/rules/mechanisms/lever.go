package main

import (
	"math"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	leverSize  = 16 // the smallest entity of the world
	leverSwing = 400 * time.Millisecond
)

// lever is a post the hero walks up to and pulls by touch, a square of its position's colour.
type lever struct{}

var (
	_ board.Figure   = lever{}
	_ board.Stateful = lever{}
	_ board.Ruled    = lever{}
)

// the lever's positions: pulled down for good, and the swings each way, the pull ending down
func (lever) Effects(b *board.Plugin) {
	fx := b.Read(0).World().Effects()
	fx.Define(LeverPulledEf, effect.Spec{})
	fx.Define(LeverPullingEf, effect.Spec{
		effect.Lasts(leverSwing),
		effect.Then(fx.Named(LeverPulledEf)),
	})
	fx.Define(LeverPushingEf, effect.Spec{effect.Lasts(leverSwing)})
}

func (lever) Rules(b *board.Plugin) { b.Read(0).World().Roles().Define(LeverRole) }

func (lever) Define(b *board.Plugin) {
	board.NewEntities(b, board.Shape{Size: leverSize}, standingAt).Define(LeverKind,
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: math.Inf(1)}), // a post: the hero walks up to it
		ceremony.Plays(b.Read(0).World().Roles().Named(LeverRole)))
}

func (lever) Look(a *world.Atlas, w *world.Plugin) {
	fx := w.Effects()
	a.Add(kind.Named[standingRow](w.Kinds(), LeverKind), leverSize, render.Solid(leverUpColor)).
		Under(fx.Named(LeverPulledEf), render.Solid(leverDownColor)).
		Under(fx.Named(LeverPullingEf), render.Solid(leverSwingColor)).
		Under(fx.Named(LeverPushingEf), render.Solid(leverSwingColor))
}
