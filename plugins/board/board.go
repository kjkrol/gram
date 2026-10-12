package board

import (
	"github.com/kjkrol/aabbworld/geom"
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
// every cell is an entity carrying a cell.Plot, a cell.Ground, a cell.Way and a cell.Built, and
// the board reads and writes them; before that, and for good on a board no ECS runs, it keeps a
// seed. The board is flat: its heights, when it has any, are its Map's (plugins/topography). Not
// safe for concurrent use.
type Board struct {
	grid.Grid

	cells   *terrain.Cells
	field   *field.Field   // the ground the others meet: cover, solid ground
	square  *grids.Square  // the grid when it is square, for the fast paths; nil otherwise
	heights bool           // the world has heights: cover spans the cells' bands
	mapping Map            // what the board is drawn and priced by; nil before the plugin set one
	decks   []terrain.Deck // scratch for the decks over a point
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
// it deciding who may and what it costs (Way.Over), and what the entities standing on it build —
// a bridge, a ford (Built.Over); off the board, the zero kind admitting nobody.
func (b *Board) Kind(c cell.ID) cell.Kind { return b.cells.Kind(c) }

// KindAt is c's kind on one of its levels where a raised Floor gives it two — on the deck, what
// the Floor carries; under it, the ground and its Way — and Kind on a cell of one level.
func (b *Board) KindAt(c cell.ID, deck bool) cell.Kind { return b.cells.KindAt(c, deck) }

// KindFor is c's kind at p, a point in c, on the level of whoever has its feet at feet and steps
// reach high: on a Floor over p — the whole cell's, or a band p lies on, reaching over from a
// neighbour too — a raised one once its feet are within reach of its deck; under the decks
// otherwise.
func (b *Board) KindFor(c cell.ID, p geom.Vec, feet, reach float64) cell.Kind {
	return b.cells.KindFor(c, p, feet, reach, b.altitude)
}

// CellFor is the cell whoever stands at p with its feet at feet and steps reach high is on: the
// cell of the highest Floor over p it stands on — a raised one within its reach, a neighbour's band
// reaching over p too — else the cell under p; false off the board. Feet at +Inf stand on any deck.
func (b *Board) CellFor(p geom.Vec, feet, reach float64) (cell.ID, bool) {
	return b.cells.CellFor(p, feet, reach, b.altitude)
}

// LevelFor is CellFor and whether whoever stands there stands on a raised deck: a point picked on
// a bridge — its feet at the deck — or on the ground under it.
func (b *Board) LevelFor(p geom.Vec, feet, reach float64) (c cell.ID, deck, ok bool) {
	return b.cells.LevelFor(p, feet, reach, b.altitude)
}

// DecksOver calls visit with every Floor over p and the cell it is built on: the whole cell's, or
// a band p lies on, the cell's own or a neighbour's reaching over it.
func (b *Board) DecksOver(p geom.Vec, visit func(c cell.ID, f cell.Floor)) {
	c, ok := b.CellAt(p)
	if !ok {
		return
	}
	b.decks = b.cells.DecksOver(c, p, b.decks[:0])
	for _, d := range b.decks {
		visit(d.Cell, d.Built.Floor)
	}
}

// NearDeck reports whether a Floor lies over any of the cell under p: its own, or a neighbour's
// band reaching into it.
func (b *Board) NearDeck(p geom.Vec) bool {
	c, ok := b.CellAt(p)
	return ok && b.cells.NearDeck(c)
}

// NarrowestDeck is how wide the narrowest Floor running as a band is; 0 for none.
func (b *Board) NarrowestDeck() float64 { return b.cells.Narrowest() }

// ThinnestDeck is how thick the thinnest raised band's slab is; 0 for none.
func (b *Board) ThinnestDeck() float64 { return b.cells.Thinnest() }

// Reaches reports whether c's Floor runs out towards its neighbour n: a band along its link to n,
// a whole floor across any side, and through a corner only along a link.
func (b *Board) Reaches(c, n cell.ID) bool { return b.cells.Reaches(c, n) }

// Fenced reports whether a railing keeping d off stands across the step from from to its neighbour
// to (cell.Rail): one on a raised deck when the step is on the decks' level, deck, else one on the
// ground's.
func (b *Board) Fenced(from, to cell.ID, d cell.Domain, deck bool) bool {
	return b.cells.Fenced(from, to, d, deck)
}

// Built is what the entities standing on c build over it — their Floors and Walls — as of the
// board's last step; none off the board.
func (b *Board) Built(c cell.ID) cell.Built { return b.cells.Built(c) }

// Bare is c's kind bare of what runs across it — a way, a bridge: the ground a step beside them
// crosses, what else the entities standing on it build included (a trapdoor, a ford); off the
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

// Along reports whether a way or a Floor's band runs from from to its neighbour to, so a step
// between them goes along it rather than over the ground beside it: the one across from links
// towards to, or to's back towards from.
func (b *Board) Along(from, to cell.ID) bool {
	out, ok := grid.Link(b.Grid, from, to)
	if !ok {
		return false
	}
	back, _ := grid.Link(b.Grid, to, from)
	if b.Way(from).Links&out != 0 || b.Way(to).Links&back != 0 {
		return true
	}
	f, t := b.Built(from).Floor, b.Built(to).Floor
	return f.Allows != 0 && f.Links&out != 0 || t.Allows != 0 && t.Links&back != 0
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
