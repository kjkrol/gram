package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Layout is a board's initial terrain for Plugin.Seed: Default fills every cell, then each of
// Cells overrides one — its kind, roles, name and group — and each of Ways lays a cell.Way across a
// cell. The ground's heights are a topography's to seed (plugins/topography); a bridge or a ford
// is an entity standing on the board (NewEntities).
type Layout struct {
	Default string
	Cells   []cell.Entry
	Ways    []cell.WayEntry
}
