package moments

import (
	"log"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/uid"
)

// Rules are the rules of a board's moments: of a unit.Standing, run for every unit on it, and of
// a cell.Now, run for every cell, every step there is one.
type Rules struct {
	grid     grid.Grid
	cells    *terrain.Cells
	tick     host.TickSource
	standing host.Rules[unit.Standing]
	now      host.Rules[cell.Now]
	rings    rings
	slope    func(p, dir geom.Vec, d cell.Domain) float64
	level    func(c cell.ID) float32

	// Log, when set, has a line written once for each unit fallen where its domain may not be.
	Log  *log.Logger
	told map[uid.UID64]bool
}

// New is the rules of the board over g with its cells, run in the Ticks tick makes. slope is the
// board's Map's, what a unit's Pace goes by besides the cell's cost, and level a cell's ground
// level.
func New(g grid.Grid, cells *terrain.Cells, tick host.TickSource,
	slope func(p, dir geom.Vec, d cell.Domain) float64, level func(c cell.ID) float32) *Rules {
	return &Rules{grid: g, cells: cells, tick: tick, slope: slope, level: level}
}

// Hosts take the rules of a unit.Standing and of a cell.Now.
func (r *Rules) Hosts() []host.Host { return []host.Host{&r.standing, &r.now} }

// StandingSystem walks every unit on the board every step: its Pace, a fall logged, the rules of
// a unit.Standing.
func (r *Rules) StandingSystem() goke.System { return newStandingSystem(r) }

// CellSystem runs the rules of a cell.Now over every cell.
func (r *Rules) CellSystem() goke.System { return newCellSystem(r) }
