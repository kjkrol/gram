package main

import (
	"time"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// grass is the land, under snow while the witch's frost lies on it
type grass struct{}

var (
	_ board.Tile     = grass{}
	_ board.Stateful = grass{}
)

func (grass) Effects(b *board.Plugin) {
	b.World().Effects().Define(FrostEf, effect.Spec{
		effect.Described("Land under snow: slower, and the witch's own ground."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows |= Frost
			g.Kind.Cost++
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
}

func (grass) Define(b *board.Plugin) {
	b.CellKinds().Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
}

func (grass) Look(a *board.Atlas, b *board.Plugin) {
	a.Add(GrassCell, render.Solid(grassColor)).
		Under(b.World().Effects().Named(FrostEf), render.Solid(snowColor))
}

// water is sailed; the cells of the lake play the lake's role (the map's legend) and freeze over
type water struct{}

var (
	_ board.Tile     = water{}
	_ board.Stateful = water{}
	_ board.Ruled    = water{}
)

func (water) Effects(b *board.Plugin) {
	b.World().Effects().Define(IcedEf, effect.Spec{
		effect.Described("Water under ice: walked over, not sailed."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows = cell.Land | Frost
			g.Kind.Cost = 2
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
}

func (water) Rules(b *board.Plugin) { b.World().Roles().Define(LakeRole) }

func (water) Define(b *board.Plugin) {
	b.CellKinds().Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
}

func (water) Look(a *board.Atlas, b *board.Plugin) {
	a.Add(WaterCell, render.Solid(waterColor)).
		Under(b.World().Effects().Named(IcedEf), render.Solid(iceColor))
}
