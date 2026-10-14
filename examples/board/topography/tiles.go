package main

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// ground is a kind of the island's ground; a snowable one has a snowy twin the winter lays.
type ground struct {
	name     string
	kind     cell.Kind
	snowable bool
}

var _ board.Tile = ground{}

var (
	dry = cell.Land | cell.Air
	wet = cell.Water | cell.Air
)

var grounds = []ground{
	{name: WaterCell, kind: cell.Kind{Cost: 1, Allows: wet}},
	{name: EarthCell, kind: cell.Kind{Cost: 2.5, Allows: dry}.Costing(cell.Air, 1), snowable: true},
	{name: SandCell, kind: cell.Kind{Cost: 4, Allows: dry}.Costing(cell.Air, 1), snowable: true},
	{name: RockCell, kind: cell.Kind{Cost: 3.25, Allows: dry}.Costing(cell.Air, 1), snowable: true},
	{name: BrookCell, kind: cell.Kind{Cost: 3.25, Allows: cell.Land | wet}.Costing(wet, 1)},
	{name: StreamCell, kind: cell.Kind{Cost: 5, Allows: cell.Land | wet}.Costing(wet, 1)},
	{name: RiverCell, kind: cell.Kind{Cost: 1, Allows: wet}},
	{name: RoadCell, kind: cell.Kind{Cost: 1, Allows: dry, Graded: true}},
	{
		name: ForestCell,
		kind: cell.Kind{
			Cost:   7.5,
			Allows: dry,
			Veil:   0.6,
			Height: scale.Units(20),
		}.Costing(cell.Air, 1),
		snowable: true,
	},
}

func (g ground) Define(b *board.Plugin) {
	b.CellKinds().Define(g.name, g.kind)
	if g.snowable {
		b.CellKinds().Define(snowyCell(g.name), g.kind)
	}
}

func (g ground) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(g.name, render.Solid(cellColors[g.name]))
	if g.snowable {
		a.Add(snowyCell(g.name), render.Solid(snowyColors[g.name]))
	}
}

// ice is what the water freezes into in the frost
type ice struct{}

var _ board.Tile = ice{}

func (ice) Define(b *board.Plugin) {
	b.CellKinds().Define(IceCell, cell.Kind{Cost: 5, Allows: dry}.Costing(cell.Air, 1))
}

func (ice) Look(a *board.Atlas, _ *board.Plugin) { a.Add(IceCell, render.Solid(iceColor)) }

func tiles() []board.Tile {
	all := make([]board.Tile, 0, len(grounds)+1)
	for _, g := range grounds {
		all = append(all, g)
	}
	return append(all, ice{})
}

func snowable() []string {
	var names []string
	for _, g := range grounds {
		if g.snowable {
			names = append(names, g.name)
		}
	}
	return names
}
