package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Layout is a board's initial terrain for Plugin.Seed: Default fills every cell, then each of
// Cells overrides one — its kind, tags, roles and wire; each of Ways lays a cell.Way across a cell, each of Crossings a
// cell.Crossing over a cell's way. The ground's heights are a topography's to seed
// (plugins/topography).
type Layout struct {
	Default   string
	Cells     []cell.Entry
	Ways      []cell.WayEntry
	Crossings []cell.WayEntry
}
