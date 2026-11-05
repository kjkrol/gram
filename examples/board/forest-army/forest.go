package main

import (
	"fmt"
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
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

const (
	forestPx = 24
	sheetPx  = 256
	// the places a sector of the forest may hold a tree at, on a lattice plotGap apart
	plots   = 144
	plotGap = 42
	// of every four plots, those planted in the forest whole and in the forest thinned
	dense  = 3
	sparse = 2
)

// three sectors across and two down, a sector 16 cells a side
var fieldSize = geom.NewVec(1500, 990)

// off the middle of a sector's part of the forest, the lattice shaken so no row lines up
var plotAt = func() (at [plots]geom.Vec) {
	shake := rand.New(rand.NewPCG(7, 11))
	side := 12
	for k := range at {
		x := float32(k%side*plotGap - (side-1)*plotGap/2)
		y := float32(k/side*plotGap - (side-1)*plotGap/2)
		at[k] = geom.NewVec(x+shake.Float32()*28-14, y+shake.Float32()*28-14)
	}
	return at
}()

type forestRow struct {
	At cell.ID `yaml:"at"`
}

// forest stands over a field of sectors: each sector come down to L3 plants its trees on the
// plots its seed picks, sparser once the forest is thinned, as one folding short of a tree
// felled thins it.
type forest struct{}

var (
	_ board.Figure   = forest{}
	_ board.Stateful = forest{}
	_ board.Ruled    = forest{}
)

func (forest) Effects(b *board.Plugin) {
	b.Read(0).World().Write().Effects().Define(ThinnedEf, effect.Spec{})
}

func (forest) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	thinned := w.Write().Effects().Named(ThinnedEf)
	standing := cond.NewWord("standing", func(f world.Fold) bool {
		return f.Parts >= plantedIn(f.ID, f.Sector, f.Box, dense)
	})
	rules := []act.Rule{act.Branch[world.Fold]{
		Name: "thin",
		When: cond.Self.If(cond.Not(thinned)).And(cond.Self.If(cond.Not(standing))),
		Then: act.Self.Put(thinned),
	}}
	sapling := kind.EntryOf(TreeKind, treeRow{})
	for k := range plots {
		tree := act.Self.Do(world.SpawnPart{Entry: sapling, At: plotAt[k]})
		rules = append(rules,
			act.Branch[world.Unfold]{
				Name: fmt.Sprintf("plant %d", k),
				When: cond.Self.If(plot(k, dense)).And(cond.Self.If(cond.Not(thinned))),
				Then: tree,
			},
			act.Branch[world.Unfold]{
				Name: fmt.Sprintf("plant %d thinned", k),
				When: cond.Self.If(plot(k, sparse)).And(cond.Self.If(thinned)),
				Then: tree,
			})
	}
	w.Roles().Define(ForestRole, rules...)
}

func (forest) Define(b *board.Plugin) {
	w := b.Read(0).World()
	kind.Define[forestRow](w.Kinds(), ForestKind, w.Roster().Spec(world.EntityTemplate,
		comp.Load(func(r forestRow) world.Position {
			c := b.Read(0).Board().CellCenter(r.At)
			from := geom.NewVec(c.X-forestPx/2, c.Y-forestPx/2)
			return world.Position{AABB: plane.NewAABB(from, forestPx, forestPx)}
		}),
		comp.Const(world.Aggregate{}),
		comp.Const(world.Field{Size: fieldSize}),
		ceremony.Plays(w.Roles().Named(ForestRole))))
}

// its trees at L2 and L3, a sheet of crowns over its field at L4, the sheet's colour at L5
func (forest) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[forestRow](w.Kinds(), ForestKind), sheetPx, unseen).
		At(lod.L4, canopy).
		At(lod.L5, render.Solid(render.MeanColour(canopy, sheetPx)))
}

// planted says whether the k-th plot of a sector of the forest whole, box its part there, holds a
// tree at density
func planted(whole uid.UID64, sector int, box geom.AABB, k int, density uint64) bool {
	mid := geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2)
	at := mid.Add(plotAt[k])
	inside := at.X >= box.TopLeft.X && at.X < box.BottomRight.X && at.Y >= box.TopLeft.Y &&
		at.Y < box.BottomRight.Y
	return inside && lod.Seed(whole, uint64(sector*plots+k))%4 < density
}

func plantedIn(whole uid.UID64, sector int, box geom.AABB, density uint64) int {
	n := 0
	for k := range plots {
		if planted(whole, sector, box, k, density) {
			n++
		}
	}
	return n
}

func plot(k int, density uint64) cond.Word[world.Unfold] {
	return cond.NewWord(fmt.Sprintf("plot %d of %d", k, density), func(u world.Unfold) bool {
		return planted(u.ID, u.Sector, u.Box, k, density)
	})
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
