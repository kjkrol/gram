package board

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Board is a Grid and its terrain. Once the ECS is set up every cell is an entity carrying a
// [Plot], a [Ground], a [Way] and a [Crossing], and the board reads and writes them; before that,
// and for good on a board no ECS runs, it keeps a seed: a TerrainMap. The board is flat: its
// heights, when it has any, are its Map's (plugins/topography). Not safe for concurrent use.
type Board struct {
	Grid

	square   *squareGrid // the grid when it is square, for the fast paths; nil otherwise
	seed     *TerrainMap
	cells    *cellStore
	version  uint64
	heights  bool        // the world has heights: cover spans the cells' bands
	mapping  Map         // what the board is drawn and priced by; nil before the plugin set one
	stamps   []uint64    // by ordinal: the count of changes when each cell last changed
	changes  uint64      // how many cells have changed, one at a time
	everyone uint64      // the count when every cell last changed at once
	boxes    []geom.AABB // scratch for the boxes of a cell
}

// cellStore is where the cells' entities are: their ids by ordinal, and a query for each of their
// components, so a read seeks only the column it needs.
type cellStore struct {
	ids       []uid.UID64
	plots     *goke.Query
	kinds     *goke.Query
	ways      *goke.Query
	crossings *goke.Query
	plot      goke.Comp[Plot]
	ground    goke.Comp[Ground]
	way       goke.Comp[Way]
	crossing  goke.Comp[Crossing]
}

var _ Terrain = (*Board)(nil)

// NewBoard is a board over grid seeded with terrain.
func NewBoard(grid Grid, terrain *TerrainMap) *Board {
	sq, _ := grid.(*squareGrid)
	return &Board{Grid: grid, square: sq, seed: terrain, stamps: make([]uint64, grid.CellCount())}
}

// CellVersion counts the changes to c — its kind, its way, its heights, through the board or by an
// effect on its entity — so whoever keeps something worked out of a cell knows when it is stale;
// it only grows, and changes to other cells leave it as it is.
func (b *Board) CellVersion(c CellID) uint64 {
	i, ok := b.ordinal(c)
	if !ok || i >= len(b.stamps) {
		return b.everyone
	}
	return max(b.stamps[i], b.everyone)
}

// Changes counts the changes to the board's cells, one at a time or all at once: while it stays
// as it was, no cell has changed.
func (b *Board) Changes() uint64 { return b.changes }

// SquareShape is a square grid's: how many columns and rows of cells how wide, and whether each
// axis wraps.
type SquareShape struct {
	Cols, Rows   uint32
	Cell         float64
	WrapX, WrapY bool
}

// Square is the board's grid's shape when it is square; false for any other.
func (b *Board) Square() (SquareShape, bool) {
	sq := b.square
	if sq == nil {
		return SquareShape{}, false
	}
	return SquareShape{Cols: sq.Width, Rows: sq.Height, Cell: float64(sq.CellSize), WrapX: sq.WrapX, WrapY: sq.WrapY}, true
}

// Touch counts a change to c made beyond the board — its heights shaped by a topography — so
// whoever keeps something worked out of the cell reads it anew (CellVersion, Version).
func (b *Board) Touch(c CellID) {
	b.touch(c)
	b.version++
}

// touch counts a change to c.
func (b *Board) touch(c CellID) {
	if i, ok := b.ordinal(c); ok && i < len(b.stamps) {
		b.changes++
		b.stamps[i] = b.changes
	}
}

// touchAll counts a change to every cell at once.
func (b *Board) touchAll() {
	b.changes++
	b.everyone = b.changes
}

// bind hands the terrain over to the cell entities in st.
func (b *Board) bind(st *cellStore) {
	b.version += b.seed.Version()
	b.cells, b.seed = st, nil
	b.touchAll()
}

// Ordinal is Grid.Ordinal, straight from the id on a square grid: c's slot in a table of one per
// cell.
func (b *Board) Ordinal(c CellID) (int, bool) { return b.ordinal(c) }

// ordinal is Grid.Ordinal, straight from the id on a square grid, whose ids count row by row.
func (b *Board) ordinal(c CellID) (int, bool) {
	if sq := b.square; sq != nil {
		return int(c), uint64(c) < uint64(sq.Width)*uint64(sq.Height)
	}
	return b.Grid.Ordinal(c)
}

// groundOf is the i-th cell's Ground, in place.
func (b *Board) groundOf(i int) *Ground {
	st := b.cells
	if !st.kinds.SeekH(st.ids[i]) && !st.kinds.Seek(st.ids[i]) {
		panic(fmt.Sprintf("board: cell entity %d is gone", st.ids[i]))
	}
	return st.ground.At(st.kinds.Cursor())
}

// CellEntity is the entity of cell c; false off the board or before the ECS is set up.
func (b *Board) CellEntity(c CellID) (uid.UID64, bool) {
	i, ok := b.ordinal(c)
	if !ok || b.cells == nil {
		return 0, false
	}
	return b.cells.ids[i], true
}

// Kind is c's terrain kind as whoever crosses it meets it: its ground's, with a Way running across
// it deciding who may and what it costs (Way.Over), and a Crossing over that letting whoever it
// admits over too (Crossing.Over); off the board, the zero kind admitting nobody.
func (b *Board) Kind(c CellID) CellKind {
	if b.cells == nil {
		return b.seed.Crossings[c].Over(b.seed.Ways[c].Over(b.seed.Kind(c)))
	}
	i, ok := b.ordinal(c)
	if !ok {
		return CellKind{}
	}
	return b.crossingOf(i).Over(b.wayOf(i).Over(b.groundOf(i).Kind))
}

// Bare is c's kind bare of what runs across it: the ground a step beside the way crosses; off the
// board, the zero kind.
func (b *Board) Bare(c CellID) CellKind {
	if b.cells == nil {
		return b.seed.Kind(c)
	}
	i, ok := b.ordinal(c)
	if !ok {
		return CellKind{}
	}
	return b.groundOf(i).Kind
}

// Set assigns c's terrain kind, taking effect immediately.
func (b *Board) Set(c CellID, kind CellKind) {
	if b.set(c, kind) {
		b.version++
	}
}

// SetMany assigns kind to every cell in cells in one call.
func (b *Board) SetMany(cells []CellID, kind CellKind) {
	changed := false
	for _, c := range cells {
		changed = b.set(c, kind) || changed
	}
	if changed {
		b.version++
	}
}

func (b *Board) set(c CellID, kind CellKind) bool {
	if b.cells == nil {
		before := b.seed.Version()
		b.seed.Set(c, kind)
		if b.seed.Version() == before {
			return false
		}
		b.touch(c)
		return true
	}
	i, ok := b.ordinal(c)
	if !ok {
		return false
	}
	g := b.groundOf(i)
	if g.Kind == kind {
		return false
	}
	g.Kind = kind
	b.touch(c)
	return true
}

// SetAll resets every cell's terrain kind to kind.
func (b *Board) SetAll(kind CellKind) {
	b.touchAll()
	if b.cells == nil {
		b.seed.SetAll(kind)
		return
	}
	st := b.cells
	for st.kinds.All(); st.kinds.Next(); {
		grounds := st.ground.Slice(st.kinds.Cursor())
		for i := range grounds {
			grounds[i].Kind = kind
		}
	}
	b.version++
}

// Version counts the changes to the terrain — kinds, ways, heights — made through the board, by an
// effect on a cell's entity or by a topography (Touch); it starts over with a load.
func (b *Board) Version() uint64 {
	if b.cells == nil {
		return b.version + b.seed.Version()
	}
	return b.version
}

// Map is what the board is drawn and priced by: the plugin's, or the simple map on a board no
// plugin runs.
func (b *Board) Map() Map {
	if b.mapping == nil {
		b.mapping = newSimpleMap(b)
	}
	return b.mapping
}

// altitude is c's ground level as the Map has it.
func (b *Board) altitude(c CellID) float64 {
	_, level := b.Map().Top(c)
	return float64(level)
}

// Cell is an entity's current position on the board.
type Cell struct{ ID CellID }

// CellAABB is the size x size world rectangle centred on c.
func CellAABB(grid Grid, c CellID, size uint32) plane.AABB {
	center := grid.CellCenter(c)
	half := float64(size) / 2
	topLeft := geom.NewVec(center.X-half, center.Y-half)
	return plane.NewAABB(topLeft, float64(size), float64(size))
}

// Center returns pos's world-space center point.
func Center(pos world.Position) geom.Vec {
	return geom.NewVec(float64(pos.TopLeft.X)+float64(pos.Size.X)/2, float64(pos.TopLeft.Y)+float64(pos.Size.Y)/2)
}
