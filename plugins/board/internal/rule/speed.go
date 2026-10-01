package rule

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
)

// TerrainSpeed is the Moving rule the board hangs on the world, its own: every entity carrying a
// unit.Mover moves at 1/CostFor(its domain) of the cell under its centre, and as slope says — the
// Map's: slower up, quicker down — unless the kind is Graded.
func TerrainSpeed(g grid.Grid, cells *terrain.Cells, slope func(p, dir geom.Vec, d cell.Domain) float64) plugin.Rule {
	return host.Each(func(_ plugin.Tick, m *unit.Mover, mv world.Moving) {
		at := mv.Base.Pos.Center()
		c, ok := g.CellAt(at)
		if !ok {
			return
		}
		kind := cells.Kind(c)
		if cost := kind.CostFor(m.Domain); cost > 0 {
			mv.Base.Vel.Value /= cost
		}
		if kind.Graded {
			return
		}
		dir := mv.Base.Vel.Dir
		if mv.Base.Vel.Value < 0 { // backing away: up or down the way it goes, not the way it faces
			dir = geom.NewVec(-dir.X, -dir.Y)
		}
		if s := slope(at, dir, m.Domain); s > 0 && s != 1 {
			mv.Base.Vel.Value /= s
		}
	})
}
