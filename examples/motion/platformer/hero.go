package main

import (
	"image/color"
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
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	heroSize  = 12
	heroSpeed = 140
	leap      = 3.5 * CellSize

	// a landing faster than this dazes, falls of four cells and more
	hardLanding = 340
	dazeLasts   = time.Second
)

type heroRow struct {
	Start cell.ID `yaml:"start"`
}

// hero is a body under gravity the hand walks and jumps, dazed by a hard landing.
type hero struct{}

var (
	_ board.Figure   = hero{}
	_ board.Stateful = hero{}
	_ board.Ruled    = hero{}
)

func (hero) Effects(b *board.Plugin) {
	b.World().Effects().Define(DazedEf, effect.Spec{
		effect.Lasts(dazeLasts),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 4 }),
	})
}

func (hero) Rules(b *board.Plugin) {
	b.World().Roles().Define(HeroRole, act.Branch[gravity.Landing]{
		Name: "daze",
		When: cond.Self.If(gravity.HarderThan(hardLanding)),
		Then: act.Self.Put(b.World().Effects().Named(DazedEf)),
	})
}

func (hero) Define(b *board.Plugin) {
	units := board.NewUnits[heroRow](b, board.Shape{Size: heroSize},
		func(u heroRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	units.Define(HeroKind,
		unit.Mover{Domain: cell.Land},
		steering.Steering{MaxSpeed: heroSpeed, Accel: heroSpeed * 8, Brake: heroSpeed * 8},
		ceremony.Plays(b.World().Roles().Named(HeroRole)),
		comp.Const(gravity.Body{Leap: leap}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
	)
}

func (hero) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[heroRow](w.Kinds(), HeroKind), heroSize, eyed(heroColor)).
		Under(w.Effects().Named(DazedEf), eyed(dazedColor))
}

func eyed(body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Solid(body)(dst, size)
		render.Dot(1, eyeColor)(dst, size)
	}
}
