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
	b.Read(0).World().Write().Effects().Define(FrostEf, effect.Spec{
		effect.Described("Land under snow: slower, and the witch's own ground."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows |= Frost
			g.Kind.Pace = 1.0 / 3
			g.Kind = g.Kind.Paced(Frost, 1)
		}),
	})
}

func (grass) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(GrassCell, cell.Kind{Pace: 0.5, Allows: cell.Land})
}

func (grass) Look(a *board.Atlas, b *board.Plugin) {
	a.Add(GrassCell, render.Solid(grassColor)).
		Under(b.Read(0).World().Write().Effects().Named(FrostEf), render.Solid(snowColor))
}

// water is sailed; the cells of the lake play the lake's role (the map's legend) and freeze over
type water struct{}

var (
	_ board.Tile     = water{}
	_ board.Stateful = water{}
	_ board.Ruled    = water{}
)

func (water) Effects(b *board.Plugin) {
	b.Read(0).World().Write().Effects().Define(IcedEf, effect.Spec{
		effect.Described("Water under ice: walked over, not sailed."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows = cell.Land | Frost
			g.Kind.Pace = 0.5
			g.Kind = g.Kind.Paced(Frost, 1)
		}),
	})
}

func (water) Rules(b *board.Plugin) { b.Read(0).World().Roles().Define(LakeRole) }

func (water) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(WaterCell, cell.Kind{Pace: 1, Allows: cell.Water})
}

func (water) Look(a *board.Atlas, b *board.Plugin) {
	a.Add(WaterCell, render.Solid(waterColor)).
		Under(b.Read(0).World().Write().Effects().Named(IcedEf), render.Solid(iceColor))
}
