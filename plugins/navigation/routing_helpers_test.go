package navigation

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/navigation/internal/traffic"
)

// routed is traffic.NewSystem with the routing answering its asks through a cell keeping of its
// own over f, as the plugin's
func routed(f *spacing.Finder, g grid.Grid, terrain cell.Terrain, occ cell.Occupancy) *traffic.System {
	s := traffic.NewSystem(f, g, terrain, occ)
	s.Routing = routing.NewCoordinator(spacing.NewCellKeeping(f, occ))
	return s
}
