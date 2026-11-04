package main

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const treePx = 20

type treeRow struct{}

// tree is a part of the forest, a post nobody walks through: a soldier striking it fells it; it
// goes as its sector folds.
type tree struct{}

var (
	_ board.Figure = tree{}
	_ board.Ruled  = tree{}
)

func (tree) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	w.Roles().Define(TreeRole,
		act.Branch[world.Folding]{
			Name: "fold in",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(world.Despawn{}),
		},
		act.Branch[collision.Meeting]{
			Name: "felled",
			When: cond.Other.If(w.Roles().Named(FellerRole)),
			Then: act.Self.Do(world.Despawn{}),
		})
}

func (tree) Define(b *board.Plugin) {
	w := b.Read(0).World()
	kind.Define[treeRow](w.Kinds(), TreeKind, w.Roster().Spec(world.EntityTemplate,
		// a part: SpawnPart lays its box off the middle of the forest's sector
		comp.Const(world.Position{
			AABB: plane.NewAABB(geom.NewVec(-treePx/2, -treePx/2), treePx, treePx),
		}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: float32(math.Inf(1))}),
		ceremony.Plays(w.Roles().Named(TreeRole))))
}

func (tree) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[treeRow](w.Kinds(), TreeKind), treePx, crown)
}

func crown(dst *render.Canvas, size int) {
	half := float32(size) / 2
	dst.FillCircle(half, half, half, crownColor)
	dst.FillCircle(half*0.75, half*0.75, half*0.45, leafColor)
	// three quarters opaque, the soldiers under it showing through: premultiplied, every channel
	// scaled alike
	for i := range dst.Pix {
		dst.Pix[i] = uint8(uint16(dst.Pix[i]) * 3 / 4)
	}
}
