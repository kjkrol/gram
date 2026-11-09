package main

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/plugins/world"
)

// woods stands over a field of the map's forest cells: each sector of it come down to L3 plants
// trees on three of every four of the plots its cells hold, the laurel of the laurisilva low, the
// maritime pine above it and the tree heather on the plateau.
type woods struct{}

var (
	_ forest.Figure = woods{}
	_ forest.Ruled  = woods{}
)

func (woods) Rules(p *forest.Plugin) {
	p.Stands().Define(ForestRole, forest.Stand{
		Cells:  []string{ForestCell, snowyCell(ForestCell)},
		Jitter: 0.25,
		Mix: []forest.Share{
			{Tree: LaurelKind, Weight: 3, To: scale.Units(800)},
			{Tree: PineKind, From: scale.Units(600)},
			{Tree: HeatherKind, Weight: 2, From: scale.Units(1300)},
		},
	})
}

func (woods) Define(p *forest.Plugin) {
	p.Types().Define(LaurelKind, laurel)
	p.Types().Define(PineKind, pine)
	p.Types().Define(HeatherKind, heather)
	w := p.World()
	kind.Define[fieldRow](w.Kinds(), ForestKind, w.Roster().Spec(world.EntityTemplate,
		append(overField(p.Board()), ceremony.Plays(w.Roles().Named(ForestRole)))...))
}

// its trees at L2 and L3; from L4 they stand in the sheet as they were planted
func (woods) Look(a *world.Atlas, p *forest.Plugin) {
	a.Add(kind.Named[fieldRow](p.World().Kinds(), ForestKind), fieldPx, unseen)
}
