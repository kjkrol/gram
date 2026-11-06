package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
)

// the plots a sector of a forest may hold a tree at: two a cell, on a lattice half a cell apart
// across and a cell down, every other column a half cell lower
const (
	treeColumns = 2 * sectorCells
	treePlots   = treeColumns * sectorCells
)

// the plots off the middle of a sector's part of a forest, each at a quarter of its cell
// whatever the part's size, a cell's two at opposite quarters
var treeLattice = func() (at [treePlots]geom.Vec) {
	for k := range at {
		i, j := k%treeColumns, k/treeColumns
		at[k] = geom.NewVec(
			float32((2*i+1)*CellSize/4-sectorCells*CellSize/2),
			float32((4*j+2*(i%2)+1)*CellSize/4-sectorCells*CellSize/2))
	}
	return at
}()

// the plots shaken within their quarters, so no row lines up
var treeShaken = func() (at [treePlots]geom.Vec) {
	shake := rand.New(rand.NewPCG(7, 11))
	for k := range at {
		q := float32(CellSize) / 8
		at[k] = treeLattice[k].Add(geom.NewVec((shake.Float32()*2-1)*q, (shake.Float32()*2-1)*q))
	}
	return at
}()

// of every four plots on the forest's cells, those planted
const planted = 3

// forest stands over a field of the map's forest cells: each sector of it come down to L3 plants
// its trees on the plots its seed picks among those on them.
type forest struct{}

var (
	_ board.Figure = forest{}
	_ board.Ruled  = forest{}
)

func (forest) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	sapling := kind.EntryOf(TreeKind, treeRow{})
	rules := make([]act.Rule, 0, treePlots)
	for k := range treePlots {
		rules = append(rules, act.Branch[world.Unfold]{
			Name: fmt.Sprintf("plant %d", k),
			When: cond.Self.If(treeAt(b, k)),
			Then: act.Self.Do(world.SpawnPart{Entry: sapling, At: treeShaken[k]}),
		})
	}
	w.Roles().Define(ForestRole, rules...)
}

func (forest) Define(b *board.Plugin) {
	w := b.Read(0).World()
	kind.Define[fieldRow](w.Kinds(), ForestKind, w.Roster().Spec(world.EntityTemplate,
		append(overField(b), ceremony.Plays(w.Roles().Named(ForestRole)))...))
}

// its trees at L2 and L3; from L4 they stand in the sheet as they were planted
func (forest) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[fieldRow](w.Kinds(), ForestKind), fieldPx, unseen)
}

// treeAt is the word of the k-th plot of a sector of a forest holding a tree: in the forest's
// part there, on a forest cell, picked by the forest's seed
func treeAt(b *board.Plugin, k int) cond.Word[world.Unfold] {
	return cond.NewWord(fmt.Sprintf("tree %d", k), func(u world.Unfold) bool {
		at := middle(u.Box).Add(treeLattice[k])
		return within(u.Box, at) && on(b, at, ForestCell) &&
			lod.Seed(u.ID, uint64(u.Sector*treePlots+k))%4 < planted
	})
}

// on reports whether at lies on a cell of the kind named, snowed on or not
func on(b *board.Plugin, at geom.Vec, name string) bool {
	bd := b.Read(0).Board()
	c, ok := bd.Grid.CellAt(at)
	if !ok {
		return false
	}
	n := bd.Bare(c).Name
	return n == cell.Named(name) || n == cell.Named(snowyCell(name))
}

func middle(box geom.AABB) geom.Vec {
	return geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2)
}

func within(box geom.AABB, at geom.Vec) bool {
	return at.X >= box.TopLeft.X && at.X < box.BottomRight.X && at.Y >= box.TopLeft.Y &&
		at.Y < box.BottomRight.Y
}
