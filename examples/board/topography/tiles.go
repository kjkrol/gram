package main

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// ground is a kind of the island's ground; a snowable one has a snowy twin the winter lays, an
// upland one plays the uplands' role, where the air rains and gathers clouds.
type ground struct {
	name     string
	kind     cell.Kind
	snowable bool
	upland   bool
}

var _ board.Tile = ground{}

var (
	dry = cell.Land | cell.Air
	wet = cell.Water | cell.Air
)

var grounds = []ground{
	{name: WaterCell, kind: cell.Kind{Pace: 1, Allows: wet}},
	{
		name:     EarthCell,
		kind:     cell.Kind{Pace: 0.4, Allows: dry, Sway: 0.35},
		snowable: true,
		upland:   true,
	},
	{name: SandCell, kind: cell.Kind{Pace: 0.25, Allows: dry}, snowable: true},
	{
		name:     RockCell,
		kind:     cell.Kind{Pace: 1 / 3.25, Allows: dry},
		snowable: true,
		upland:   true,
	},
	{name: BrookCell, kind: cell.Kind{Pace: 1 / 3.25, Allows: cell.Land | wet}.Paced(wet, 1)},
	{name: StreamCell, kind: cell.Kind{Pace: 0.2, Allows: cell.Land | wet}.Paced(wet, 1)},
	{name: RiverCell, kind: cell.Kind{Pace: 1, Allows: wet}},
	{name: RoadCell, kind: cell.Kind{Pace: 1, Allows: dry, Graded: true}},
	{
		name: ForestCell,
		kind: cell.Kind{
			Pace:   2.0 / 15,
			Allows: dry,
			Veil:   0.6,
			Height: scale.Units(20),
			Sway:   0.6,
		},
		snowable: true,
		upland:   true,
	},
}

func (g ground) Define(b *board.Plugin) {
	var plays []*ceremony.Part
	if g.upland {
		plays = append(plays, b.Read(0).World().Roles().Named(UplandRole))
	}
	b.Read(0).CellKinds().Define(g.name, g.kind, plays...)
	if g.snowable {
		b.Read(0).CellKinds().Define(snowyCell(g.name), g.kind, plays...)
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
	b.Read(0).CellKinds().Define(IceCell, cell.Kind{Pace: 0.2, Allows: dry})
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
