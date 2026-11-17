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
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/buildings"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// the village's own box, world units: it is drawn by its buildings
const villagePx = 16

type villageRow struct {
	From cell.ID `yaml:"from"`
	To   cell.ID `yaml:"to"`
}

// village is the field over the cells From to To its buildings stand on: a sector of it come
// near raises them as entities, each casting its shadow, and they go as it folds far off, where
// the plugin draws their bodies alone.
type village struct{}

var (
	_ board.Figure = village{}
	_ board.Ruled  = village{}
)

func (village) Rules(b *board.Plugin) {
	b.Read(0).World().Roles().Define(VillageRole, act.Branch[world.Unfold]{
		Name: "its buildings stand",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(buildings.Raise{}),
	})
}

func (village) Define(b *board.Plugin) {
	w := b.Read(0).World()
	corners := func(r villageRow) (geom.Vec, geom.Vec) {
		bd := b.Read(0).Board()
		return bd.CellCenter(r.From), bd.CellCenter(r.To)
	}
	kind.Define[villageRow](w.Kinds(), VillageKind, w.Roster().Spec(world.EntityTemplate,
		comp.Load(func(r villageRow) world.Position {
			from, to := corners(r)
			corner := geom.NewVec((from.X+to.X-villagePx)/2, (from.Y+to.Y-villagePx)/2)
			return world.Position{AABB: plane.NewAABB(corner, villagePx, villagePx)}
		}),
		comp.Load(func(r villageRow) world.Field {
			from, to := corners(r)
			return world.Field{Size: geom.NewVec(to.X-from.X+CellSize, to.Y-from.Y+CellSize)}
		}),
		comp.Const(world.Aggregate{}),
		ceremony.Plays(w.Roles().Named(VillageRole))))
}

func (village) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[villageRow](w.Kinds(), VillageKind), villagePx, func(*render.Canvas, int) {})
}
