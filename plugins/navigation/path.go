package navigation

import "github.com/kjkrol/gram/plugins/board/cell"

// MaxPathLength bounds Path.Steps — a route longer than this is fetched in successive chunks.
const MaxPathLength = 64

// Path is the cached route toward MoveOrder.Target, consumed step by step by
// NavigationSystem — Length 0 means "not computed yet".
type Path struct {
	Steps  [MaxPathLength]cell.ID
	Length uint16
	Index  uint16
}
