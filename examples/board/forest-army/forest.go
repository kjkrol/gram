package main

import (
	"math/rand/v2"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	forestPx = 24
	sheetPx  = 256
	// trees a cell in the forest whole and in the forest thinned
	dense  = 0.42
	sparse = 0.28
)

// three sectors across and two down, a sector 16 cells a side
var fieldSize = geom.NewVec(1500, 990)

type forestRow struct {
	At cell.ID `yaml:"at"`
}

// woods stands over a field of sectors: each sector come down to L3 plants its trees, sparser once
// the forest is thinned, as one folding short of a tree felled thins it.
type woods struct{}

var (
	_ forest.Figure   = woods{}
	_ forest.Stateful = woods{}
	_ forest.Ruled    = woods{}
)

func (woods) Effects(p *forest.Plugin) {
	p.World().Write().Effects().Define(ThinnedEf, effect.Spec{})
}

func (woods) Rules(p *forest.Plugin) {
	w := p.World()
	thinned := w.Write().Effects().Named(ThinnedEf)
	whole := forest.Stand{
		Density: dense,
		Jitter:  0.5,
		Mix:     []forest.Share{{Tree: TreeKind}},
		When:    cond.Self.If(cond.Not(thinned)),
	}
	thin := whole
	thin.Density = sparse
	thin.When = cond.Self.If(thinned)
	p.Stands().Define(ForestRole, whole, act.Branch[world.Fold]{
		Name: "thin",
		When: cond.Self.If(cond.Not(thinned)).And(cond.Self.If(p.Stands().Felled(whole))),
		Then: act.Self.Put(thinned),
	})
	p.Stands().Define(ThinnedRole, thin)
	w.Roles().Define(TreeRole, felled(w))
}

func (woods) Define(p *forest.Plugin) {
	p.Types().Define(TreeKind, tree)
	w := p.World()
	kind.Define[forestRow](w.Kinds(), ForestKind, w.Roster().Spec(world.EntityTemplate,
		comp.Load(func(r forestRow) world.Position {
			c := p.Board().Read(0).Board().CellCenter(r.At)
			from := geom.NewVec(c.X-forestPx/2, c.Y-forestPx/2)
			return world.Position{AABB: plane.NewAABB(from, forestPx, forestPx)}
		}),
		comp.Const(world.Aggregate{}),
		comp.Const(world.Field{Size: fieldSize}),
		ceremony.Plays(w.Roles().Named(ForestRole), w.Roles().Named(ThinnedRole))))
}

// its trees at L2 and L3, a sheet of crowns over its field at L4, the sheet's colour at L5
func (woods) Look(a *world.Atlas, p *forest.Plugin) {
	a.Add(kind.Named[forestRow](p.World().Kinds(), ForestKind), sheetPx, unseen).
		At(lod.L4, canopy).
		At(lod.L5, render.Solid(render.MeanColour(canopy, sheetPx)))
}

func unseen(*render.Canvas, int) {}

// crowns seen from above over the forest floor, as many as a sector's plots bear
func canopy(dst *render.Canvas, size int) {
	dst.FillRect(0, 0, float32(size), float32(size), floorColor)
	scatter := rand.New(rand.NewPCG(3, 5))
	for range size * size / 16 {
		x, y := scatter.Float32()*float32(size), scatter.Float32()*float32(size)
		r := 1 + scatter.Float32()*1.2
		dst.FillCircle(x, y, r, crownColor)
		dst.FillCircle(x-r/3, y-r/3, r/2, leafColor)
	}
}
