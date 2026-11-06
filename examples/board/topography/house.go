package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const housePx = 14

type houseRow struct{}

// house is a part of a settlement, a whitewashed house under a tiled roof: it goes as its sector
// folds.
type house struct{}

var (
	_ board.Figure = house{}
	_ board.Ruled  = house{}
)

func (house) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	w.Roles().Define(HouseRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (house) Define(b *board.Plugin) {
	w := b.Read(0).World()
	kind.Define[houseRow](w.Kinds(), HouseKind, w.Roster().Spec(board.StandingTemplate,
		// a part: SpawnPart lays its box off the middle of the settlement's sector
		comp.Const(world.Position{
			AABB: plane.NewAABB(geom.NewVec(-housePx/2, -housePx/2), housePx, housePx),
		}),
		comp.Const(world.Z{Height: scale.Figure(6)}),
		ceremony.Plays(w.Roles().Named(HouseRole))))
}

func (house) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[houseRow](w.Kinds(), HouseKind), housePx, roof).Upright()
}

// a roof seen from above, its two slopes either side of the ridge, the walls' tops round it
func roof(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillRect(0, 0, s, s, wallColor)
	dst.FillRect(1, 1, s-2, s/2-1, roofColor)
	dst.FillRect(1, s/2, s-2, s/2-1, roofShade)
	dst.FillRect(1, s/2-1, s-2, 1, ridgeColor)
}
