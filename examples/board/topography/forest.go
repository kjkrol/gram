package main

import (
	"maps"
	"slices"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/plugins/world"
)

// growth stands over a field of the map's cells the land cover has something growing on: each
// sector of it come down to L3 plants, on the plots its cells of each kind hold, that kind's
// stand.
type growth struct{}

var (
	_ forest.Figure = growth{}
	_ forest.Ruled  = growth{}
)

// stand is how one kind of the land cover's cells is planted, and the role that plants it
type stand struct {
	role  string
	stand forest.Stand
}

// what grows on Madeira by the land cover: the laurisilva of the north, its trees by the height
// of the ground; the woods of the south, eucalyptus and acacia brought in, the chestnut and the
// pine above them; the pinewoods; the heath of the heights, thin; the terraces, bananas by the
// sea and vines above them.
var stands = []stand{
	{role: LaurisilvaRole, stand: forest.Stand{
		Cells:  growing(LaurisilvaCell),
		Jitter: 0.25,
		Mix: []forest.Share{
			{Tree: BarbusanoKind, Weight: 2, To: scale.Units(500)},
			{Tree: LaurelKind, Weight: 3, From: scale.Units(300), To: scale.Units(1200)},
			{Tree: TilKind, Weight: 3, From: scale.Units(500), To: scale.Units(1100)},
			{Tree: VinhaticoKind, Weight: 2, From: scale.Units(400), To: scale.Units(1000)},
			{Tree: FaiaKind, From: scale.Units(800)},
			{Tree: HeatherKind, Weight: 2, From: scale.Units(1100)},
		},
	}},
	{role: WoodlandRole, stand: forest.Stand{
		Cells:  growing(WoodlandCell),
		Jitter: 0.25,
		Mix: []forest.Share{
			{Tree: EucalyptusKind, Weight: 3, To: scale.Units(900)},
			{Tree: AcaciaKind, Weight: 2, From: scale.Units(200), To: scale.Units(900)},
			{Tree: ChestnutKind, From: scale.Units(400), To: scale.Units(1000)},
			{Tree: PineKind, From: scale.Units(500)},
			{Tree: FaiaKind, From: scale.Units(700)},
		},
	}},
	{role: PinewoodRole, stand: forest.Stand{
		Cells:  growing(PinewoodCell),
		Jitter: 0.25,
		Mix: []forest.Share{
			{Tree: PineKind, Weight: 4},
			{Tree: CedarKind, From: scale.Units(1200)},
		},
	}},
	{role: HeathRole, stand: forest.Stand{
		Cells:   growing(HeathCell),
		Density: 0.5,
		Jitter:  0.4,
		Mix: []forest.Share{
			{Tree: HeatherKind, Weight: 6},
			{Tree: CedarKind, From: scale.Units(1400)},
		},
	}},
	{role: FarmlandRole, stand: forest.Stand{
		Cells:   growing(FarmlandCell),
		Density: 0.75,
		Jitter:  0.1,
		Mix: []forest.Share{
			{Tree: BananaKind, To: scale.Units(250)},
			{Tree: VineKind, From: scale.Units(250), To: scale.Units(700)},
			{Tree: ChestnutKind, From: scale.Units(700)},
		},
	}},
}

// growing is a kind of cells and its snowy twin
func growing(cells string) []string { return []string{cells, snowyCell(cells)} }

func (growth) Rules(p *forest.Plugin) {
	for _, s := range stands {
		p.Stands().Define(s.role, s.stand)
	}
}

func (growth) Define(p *forest.Plugin) {
	for _, name := range slices.Sorted(maps.Keys(trees)) {
		p.Types().Define(name, trees[name])
	}
	w := p.World()
	plays := make([]*ceremony.Part, len(stands))
	for i, s := range stands {
		plays[i] = w.Roles().Named(s.role)
	}
	kind.Define[fieldRow](w.Kinds(), GrowthKind, w.Roster().Spec(world.EntityTemplate,
		append(overField(p.Board()), ceremony.Plays(plays...))...))
}

// its trees at L2 and L3; from L4 they stand in the sheet as they were planted
func (growth) Look(a *world.Atlas, p *forest.Plugin) {
	a.Add(kind.Named[fieldRow](p.World().Kinds(), GrowthKind), fieldPx, unseen)
}
