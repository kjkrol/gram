package board

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
)

// terrainSpeed is the Moving behavior board registers on the world: every entity carrying a Mover
// moves at 1/CostFor(its domain) of the cell under its centre, and slower up a slope and quicker down
// it, as the board's Climbing says.
func terrainSpeed(brd *Board) plugin.Behavior {
	return host.Each[Mover](func(_ plugin.Tick, m *Mover, mv world.Moving) {
		cell, ok := brd.CellAt(Center(mv.Base.Pos))
		if !ok {
			return
		}
		if cost := brd.Kind(cell).CostFor(m.Domain); cost > 0 {
			mv.Base.Vel.Value /= cost
		}
		if brd.climbing.Feels(m.Domain) {
			mv.Base.Vel.Value /= brd.climbing.Factor(brd.slopeAt(Center(mv.Base.Pos), mv.Base.Vel.Dir))
		}
	})
}
