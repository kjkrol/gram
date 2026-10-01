package rule

import (
	"errors"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
)

// Rules are the rules hooked on a board: of a unit.Standing, run for every unit on it, and of a
// cell.Now, run for every cell, every step one is hooked.
type Rules struct {
	grid     grid.Grid
	cells    *terrain.Cells
	tick     plugin.TickSource // the world's
	standing host.EachHost[unit.Standing]
	now      host.EachHost[cell.Now]
	rings    rings
}

// New is the rules of the board over g with its cells, run in the Ticks tick makes.
func New(g grid.Grid, cells *terrain.Cells, tick plugin.TickSource) *Rules {
	return &Rules{grid: g, cells: cells, tick: tick}
}

// Hook hosts r, a rule of a unit.Standing or of a cell.Now; plugin.ErrUnhosted for any other.
func (r *Rules) Hook(rule plugin.Rule) error {
	err := r.standing.Add(rule)
	if errors.Is(err, plugin.ErrUnhosted) {
		err = r.now.Add(rule)
	}
	return err
}

// StandingSystem runs the rules of a unit.Standing over every unit on the board.
func (r *Rules) StandingSystem() goke.System { return newStandingSystem(r) }

// CellSystem runs the rules of a cell.Now over every cell.
func (r *Rules) CellSystem() goke.System { return newCellSystem(r) }
