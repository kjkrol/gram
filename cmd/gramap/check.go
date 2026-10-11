package main

import (
	"testing/fstest"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

type crossingRow struct {
	At     cell.ID `yaml:"at"`
	Height float64 `yaml:"height"`
	Way    [2]int  `yaml:"way"`
}

// check lays the map — its units left out, the game's — on a board as a game would, so a road
// lying where no road may, or anything else a board refuses, fails here.
func (w *chart) check(data []byte) error {
	units := w.r.Area.Units
	wp := world.NewPlugin(world.Config{
		Space: world.SpaceCfg{Width: uint32(w.cols) * units, Height: uint32(w.rows) * units},
		Entities: world.EntitiesCfg{
			MaxCount: w.cols * w.rows,
			MinSize:  1,
			MaxSize:  units,
		},
		Heights: !w.flat,
		Scale:   world.Scale{Metres: w.perUnit},
	})
	g := grid.DefaultGrids{}.Square(uint32(w.cols), uint32(w.rows), uint32(units))
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, wp)
	k := w.r.Kinds
	ground := cell.Land | cell.Air
	kinds := b.CellKinds()
	kinds.Define(k.Sea, cell.Kind{Cost: 1, Allows: cell.Water | cell.Air})
	for _, name := range []string{k.Earth, k.Sand, k.Rock, k.Forest} {
		kinds.Define(name, cell.Kind{Cost: 1, Allows: ground})
	}
	for _, name := range []string{k.Brook, k.Stream, k.River} {
		if _, ok := kinds.Get(name); !ok {
			kinds.Define(name, cell.Kind{Cost: 1, Allows: cell.Water | cell.Air})
		}
	}
	kinds.Define(k.Road, cell.Kind{Cost: 1, Allows: ground, Graded: true})
	standing := board.NewEntities(b, board.Shape{Size: w.size},
		func(r crossingRow) cell.ID { return r.At })
	standing.Define(k.Bridge)
	if k.Ford != k.Bridge {
		standing.Define(k.Ford)
	}
	_, err := b.Lay(fstest.MapFS{"map.yaml": {Data: data}}, "map.yaml")
	return err
}
