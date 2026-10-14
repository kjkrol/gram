package main

import (
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board/cell"
)

var snowable = []string{EarthCell, SandCell, RockCell, ForestCell}

func (a *arena) defineWinterCells() {
	kinds := a.board.CellKinds()
	for _, name := range snowable {
		k, _ := kinds.Get(name)
		kinds.Define(snowyCell(name), k)
	}
	kinds.Define(IceCell, cell.Kind{Cost: 5, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
}

func (a *arena) defineClimate() weathering.Config {
	snowy := map[string]string{}
	for _, name := range snowable {
		snowy[name] = snowyCell(name)
	}
	return weathering.Config{
		Snowy: snowy,
		Ice:   IceCell,
		Sway:  []string{ForestCell, snowyCell(ForestCell)},
	}
}
