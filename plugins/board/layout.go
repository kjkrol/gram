package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Layout is a board's initial terrain for Plugin.Seed: Default fills every cell, each of Cells
// overrides one and each of Ways lays a cell.Way across a cell.
type Layout struct {
	Default string
	Cells   []cell.Entry
	Ways    []cell.WayEntry
}
