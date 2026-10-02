package moments

import (
	"errors"
	"log"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// Rules are the rules hooked on a board: of a unit.Standing, run for every unit on it, and of a
// cell.Now, run for every cell, every step one is hooked.
type Rules struct {
	grid     grid.Grid
	cells    *terrain.Cells
	tick     plugin.TickSource // the world's
	standing plugin.Rules[unit.Standing]
	now      plugin.Rules[cell.Now]
	rings    rings
	slope    func(p, dir geom.Vec, d cell.Domain) float64 // the Map's, for every unit's Pace

	// Log has a line written, once for each, for a unit fallen where its domain may not be — in a
	// hole, a walker in the water; nil for none.
	Log  *log.Logger
	told map[uid.UID64]bool
}

// New is the rules of the board over g with its cells, run in the Ticks tick makes; slope is the
// board's Map's, what a unit's Pace goes by besides the cost of the cell under it.
func New(g grid.Grid, cells *terrain.Cells, tick plugin.TickSource, slope func(p, dir geom.Vec, d cell.Domain) float64) *Rules {
	return &Rules{grid: g, cells: cells, tick: tick, slope: slope}
}

// Hook hosts r, a rule of a unit.Standing or of a cell.Now; plugin.ErrUnhosted for any other.
func (r *Rules) Hook(b rule.Rule) error {
	err := r.standing.Add(b)
	if errors.Is(err, plugin.ErrUnhosted) {
		err = r.now.Add(b)
	}
	return err
}

// StandingSystem walks every unit on the board in every step: it writes its Pace, the ground's,
// logs a fall when told to and runs the rules of a unit.Standing.
func (r *Rules) StandingSystem() goke.System { return newStandingSystem(r) }

// CellSystem runs the rules of a cell.Now over every cell.
func (r *Rules) CellSystem() goke.System { return newCellSystem(r) }
