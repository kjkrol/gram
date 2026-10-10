package main

import (
	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// snowable are the kinds snow may lie on, each with a snowy twin.
var snowable = []string{island.EarthCell, island.SandCell, island.RockCell, island.ForestCell}

// defineWinterCells defines the winter's kinds: each ground's snowy twin, and ice.
func (s *arena) defineWinterCells() {
	kinds := s.board.CellKinds()
	for _, name := range snowable {
		k, _ := kinds.Get(name)
		kinds.Define(snowyCell(name), k)
	}
	kinds.Define(IceCell, cell.Kind{Cost: 5, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
}

// defineClimate is what the weather does to the island, all by the kinds' names: the winter's
// kinds themselves are defined in the Kinds section, later.
func (s *arena) defineClimate() weathering.Config {
	snowy := map[string]string{}
	for _, name := range snowable {
		snowy[name] = snowyCell(name)
	}
	return weathering.Config{
		Snowy: snowy,
		Ice:   IceCell,
		Sway:  []string{island.ForestCell, snowyCell(island.ForestCell)},
	}
}
