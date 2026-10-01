package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
)

// terrainSpeed is the Moving hook board hangs on the world, its own: every entity carrying a Mover
// moves at 1/CostFor(its domain) of the cell under its centre, and as the Map's slope under it
// says — slower up, quicker down — unless the kind is Graded.
func terrainSpeed(brd *Board) plugin.Rule {
	return host.Each(func(_ plugin.Tick, m *Mover, mv world.Moving) {
		at := Center(mv.Base.Pos)
		cell, ok := brd.CellAt(at)
		if !ok {
			return
		}
		kind := brd.Kind(cell)
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
		if slope := brd.Map().Slope(at, dir, m.Domain); slope > 0 && slope != 1 {
			mv.Base.Vel.Value /= slope
		}
	})
}
