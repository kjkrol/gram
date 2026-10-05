package board

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/field"
	"github.com/kjkrol/gram/plugins/board/internal/grids"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Board is a Grid and its terrain, the one place to read and write it. Once the ECS is set up
// every cell is an entity carrying a cell.Plot, a cell.Ground, a cell.Way and a cell.Crossing, and
// the board reads and writes them; before that, and for good on a board no ECS runs, it keeps a
// seed. The board is flat: its heights, when it has any, are its Map's (plugins/topography). Not
// safe for concurrent use.
type Board struct {
	grid.Grid

	cells   *terrain.Cells
	field   *field.Field  // the ground the others meet: cover, solid ground
	square  *grids.Square // the grid when it is square, for the fast paths; nil otherwise
	heights bool          // the world has heights: cover spans the cells' bands
	mapping Map           // what the board is drawn and priced by; nil before the plugin set one
}

var _ cell.Terrain = (*Board)(nil)

// NewBoard is a board over g, every cell of the zero kind until it is set.
func NewBoard(g grid.Grid) *Board {
	sq, _ := g.(*grids.Square)
	b := &Board{Grid: g, cells: terrain.New(g), square: sq}
	b.field = field.New(g, b.cells, b.altitude)
	return b
}

// setHeights says whether the world has heights.
func (b *Board) setHeights(heights bool) {
	b.heights = heights
	b.field.SetHeights(heights)
}

// Shape is the board's grid's shape when it is one of grid.DefaultGrids; false for any other.
func (b *Board) Shape() (grid.Shape, bool) { return grid.ShapeOf(b.Grid) }

// Ordinal is Grid.Ordinal, straight from the id on a square grid: c's slot in a table of one per
// cell.
func (b *Board) Ordinal(c cell.ID) (int, bool) { return b.cells.Ordinal(c) }

// CellEntity is the entity of cell c; false off the board or before the ECS is set up.
func (b *Board) CellEntity(c cell.ID) (uid.UID64, bool) { return b.cells.Entity(c) }

// Kind is c's terrain kind as whoever crosses it meets it: its ground's, with a Way running across
// it deciding who may and what it costs (Way.Over), and a Crossing over that letting whoever it
// admits over too (Crossing.Over); off the board, the zero kind admitting nobody.
func (b *Board) Kind(c cell.ID) cell.Kind { return b.cells.Kind(c) }

// Bare is c's kind bare of what runs across it: the ground a step beside the way crosses; off the
// board, the zero kind.
func (b *Board) Bare(c cell.ID) cell.Kind { return b.cells.Bare(c) }

// Set assigns c's terrain kind, taking effect immediately.
func (b *Board) Set(c cell.ID, kind cell.Kind) { b.cells.Set(c, kind) }

// SetAll resets every cell's terrain kind to kind.
func (b *Board) SetAll(kind cell.Kind) { b.cells.SetAll(kind) }

// States are the markers of the effects on c now: what lies on the cell, for whoever draws it.
func (b *Board) States(c cell.ID) tag.Tags[effect.States] { return b.cells.States(c) }

// Way is what runs across c; the zero Way off the board or where nothing does.
func (b *Board) Way(c cell.ID) cell.Way { return b.cells.Way(c) }

// SetWay lays w across c, the zero Way taking what ran there away; it takes effect immediately.
func (b *Board) SetWay(c cell.ID, w cell.Way) { b.cells.SetWay(c, w) }

// Crossing is what crosses c over its Way; the zero Crossing off the board or where nothing does.
func (b *Board) Crossing(c cell.ID) cell.Crossing { return b.cells.Crossing(c) }

// SetCrossing lays x across c over its Way, the zero Crossing taking what crossed there away; it
// takes effect immediately.
func (b *Board) SetCrossing(c cell.ID, x cell.Crossing) { b.cells.SetCrossing(c, x) }

// Along reports whether a way runs from from to its neighbour to, so a step between them goes
// along it rather than over the ground beside it: the way or the crossing across from links
// towards to, or to's back towards from.
func (b *Board) Along(from, to cell.ID) bool {
	out, ok := grid.Link(b.Grid, from, to)
	if !ok {
		return false
	}
	back, _ := grid.Link(b.Grid, to, from)
	return b.Way(from).Links&out != 0 || b.Way(to).Links&back != 0 ||
		b.Crossing(from).Links&out != 0 || b.Crossing(to).Links&back != 0
}

// CellVersion counts the changes to c — its kind, its way, its heights, through the board or by an
// effect on its entity — so whoever keeps something worked out of a cell knows when it is stale;
// it only grows, and changes to other cells leave it as it is.
func (b *Board) CellVersion(c cell.ID) uint64 { return b.cells.CellVersion(c) }

// Changes counts the changes to the board's cells, one at a time or all at once: while it stays
// as it was, no cell has changed.
func (b *Board) Changes() uint64 { return b.cells.Changes() }

// Version counts the changes to the terrain — kinds, ways, heights — made through the board, by an
// effect on a cell's entity or by a topography (Touch); it starts over with a load.
func (b *Board) Version() uint64 { return b.cells.Version() }

// Touch counts a change to c made beyond the board — its heights shaped by a topography — so
// whoever keeps something worked out of the cell reads it anew (CellVersion, Version).
func (b *Board) Touch(c cell.ID) { b.cells.Touch(c) }

// Map is what the board is drawn and priced by: the plugin's, or the simple map on a board no
// plugin runs.
func (b *Board) Map() Map {
	if b.mapping == nil {
		b.mapping = newSimpleMap(b)
	}
	return b.mapping
}

// altitude is c's ground level as the Map has it.
func (b *Board) altitude(c cell.ID) float64 {
	_, level := b.Map().Top(c)
	return float64(level)
}
