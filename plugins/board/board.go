package board

import (
	"encoding/binary"
	"hash/fnv"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/field"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/uid"
)

// board is a Grid and its terrain, written through its BoardWrite on the game's goroutine and read
// through a BoardRead on any, a BoardRead to a goroutine
type board struct {
	grid.Grid

	cells   *terrain.Cells
	field   *field.Field // the main read's
	mapping Map
	read    BoardRead
	write   BoardWrite
	drawn   *drawn
}

// NewBoard is a board over g with no plugin, every cell of the zero kind until it is set: what
// reads it and what writes it.
func NewBoard(g grid.Grid) (*BoardRead, *BoardWrite) {
	b := newBoard(g)
	return &b.read, &b.write
}

func newBoard(g grid.Grid) *board {
	b := &board{Grid: g, cells: terrain.New(g)}
	b.read = BoardRead{Grid: g, b: b, cells: b.cells.Read()}
	b.write.b = b
	b.drawn = newDrawn(b)
	b.mapping = newSimpleMap(b.drawnBoard())
	b.field = field.New(g, b.read.cells, b.read.altitude)
	b.read.cover = b.field
	b.cells.Notify(b.field.Veil, b.field.Veils)
	return b
}

// drawnBoard is the board as the frame drawn reads it.
func (b *board) drawnBoard() drawnBoard { return drawnBoard{Grid: b.Grid, d: b.drawn} }

// BoardRead reads a board, its grid and its terrain, on one goroutine at a time: the main one's
// through the board's own queries, a Fork's through queries of its own.
type BoardRead struct {
	grid.Grid

	b     *board
	cells *terrain.Read
	cover *field.Field
}

var _ cell.Terrain = (*BoardRead)(nil)

// Fork reads the same board through queries and scratch of its own, for another goroutine.
func (r *BoardRead) Fork() *BoardRead {
	f := &BoardRead{Grid: r.Grid, b: r.b, cells: r.cells.Fork()}
	f.cover = r.b.field.Fork(f.cells, f.altitude)
	return f
}

// Shape is the board's grid's shape when it is one of grid.DefaultGrids; false for any other.
func (r *BoardRead) Shape() (grid.Shape, bool) { return grid.ShapeOf(r.Grid) }

// Ordinal is c's slot in a table of one per cell.
func (r *BoardRead) Ordinal(c cell.ID) (int, bool) { return r.cells.Ordinal(c) }

// CellEntity is the entity of cell c; false off the board or before the ECS is set up.
func (r *BoardRead) CellEntity(c cell.ID) (uid.UID64, bool) { return r.cells.Entity(c) }

// Kind is c's terrain kind as whoever crosses it meets it, its Way and what is built over it
// included; off the board, the zero kind admitting nobody.
func (r *BoardRead) Kind(c cell.ID) cell.Kind { return r.cells.Kind(c) }

// KindAt is c's kind on level; Kind on a cell of one level.
func (r *BoardRead) KindAt(c cell.ID, level cell.Level) cell.Kind {
	return r.cells.KindAt(c, level)
}

// KindFor is c's kind at p, a point in c, on the level of whoever has its feet at feet and steps
// reach high: a deck over p within reach, else what lies under.
func (r *BoardRead) KindFor(c cell.ID, p geom.Vec, feet, reach float32) cell.Kind {
	return r.cells.KindFor(c, p, feet, reach, r.altitude)
}

// CellFor is the cell whoever stands at p with its feet at feet and steps reach high is on: the
// cell of the highest deck over p within reach, else the cell under p. Feet at +Inf stand on any
// deck.
func (r *BoardRead) CellFor(p geom.Vec, feet, reach float32) (cell.ID, bool) {
	return r.cells.CellFor(p, feet, reach, r.altitude)
}

// LevelFor is CellFor and the level whoever stands there stands on.
func (r *BoardRead) LevelFor(p geom.Vec, feet, reach float32) (c cell.ID, level cell.Level,
	ok bool) {
	return r.cells.LevelFor(p, feet, reach, r.altitude)
}

func (r *BoardRead) footing(p geom.Vec, feet, reach float32) (c cell.ID, level cell.Level,
	raised, ok bool) {
	if c, level, ok = r.LevelFor(p, feet, reach); !ok {
		return c, cell.OnGround, false, false
	}
	b := r.Built(c)
	return c, level, b.Levelled(), true
}

// DecksOver calls visit with every Floor over p and the cell it is built on, a neighbour's band
// reaching over p too.
func (r *BoardRead) DecksOver(p geom.Vec, visit func(c cell.ID, f cell.Floor)) {
	c, ok := r.CellAt(p)
	if !ok {
		return
	}
	r.cells.DecksOver(c, p, func(d terrain.Deck) bool {
		visit(d.Cell, d.Built.Floor)
		return true
	})
}

// Storeyed reports whether a floor gives any cell of the board a level beside its ground.
func (r *BoardRead) Storeyed() bool { return r.cells.Storeyed() }

// Sunk reports whether a floor lies sunk under any cell of the board.
func (r *BoardRead) Sunk() bool { return r.cells.Sunk() }

// NearDeck reports whether a Floor lies over any of the cell under p.
func (r *BoardRead) NearDeck(p geom.Vec) bool {
	c, ok := r.CellAt(p)
	return ok && r.cells.NearDeck(c)
}

// NarrowestDeck is how wide the narrowest Floor running as a band is; 0 for none.
func (r *BoardRead) NarrowestDeck() float32 { return r.cells.Narrowest() }

// ThinnestDeck is how thick the thinnest raised band's slab is; 0 for none.
func (r *BoardRead) ThinnestDeck() float32 { return r.cells.Thinnest() }

// Reaches reports whether c's Floor runs out towards its neighbour n; through a corner only
// along a link.
func (r *BoardRead) Reaches(c, n cell.ID) bool { return r.cells.Reaches(c, n) }

// ReachesOn is Reaches of the floor making c's level.
func (r *BoardRead) ReachesOn(c, n cell.ID, level cell.Level) bool {
	return r.cells.ReachesOn(c, n, level)
}

// Lane is how wide a body may be across the step from from to its neighbour to in d where a
// deck's band runs along it on level; +Inf where no band narrows the step. The railings keeping
// d off are taken off the band's width.
func (r *BoardRead) Lane(from, to cell.ID, d cell.Domain, level cell.Level) float32 {
	return r.cells.Lane(from, to, d, level)
}

// Fenced reports whether a railing keeping d off stands across the step from from to its
// neighbour to on level.
func (r *BoardRead) Fenced(from, to cell.ID, d cell.Domain, level cell.Level) bool {
	return r.cells.Fenced(from, to, d, level)
}

// Railed reports whether a railing keeping d off runs along c's own band on level: a band
// running on through c, a head carrying none.
func (r *BoardRead) Railed(c cell.ID, d cell.Domain, level cell.Level) bool {
	return r.cells.Railed(c, d, level)
}

// Limited reports whether a floor's Load or a wall's Gap limits whom a cell takes anywhere on the
// board.
func (r *BoardRead) Limited() bool { return r.cells.Limited() }

// Limits are what c limits a body moving in d by beyond its kind on level:
// the Load of the floor carrying it where its ground would not, and the Gap of the wall keeping d
// off; zero where nothing does.
func (r *BoardRead) Limits(c cell.ID, d cell.Domain, level cell.Level) (load float32,
	gap cell.Gap) {
	return r.cells.Limits(c, d, level)
}

// Built is what the entities standing on c build over it as of the board's last step; none off
// the board.
func (r *BoardRead) Built(c cell.ID) cell.Built { return r.cells.Built(c) }

// Bare is c's kind bare of what runs across it, a way or a bridge: the ground a step beside them
// crosses; off the board, the zero kind.
func (r *BoardRead) Bare(c cell.ID) cell.Kind { return r.cells.Bare(c) }

// States are the markers of the effects on c now.
func (r *BoardRead) States(c cell.ID) tag.Tags[effect.States] { return r.cells.States(c) }

// Way is what runs across c; the zero Way off the board or where nothing does.
func (r *BoardRead) Way(c cell.ID) cell.Way { return r.cells.Way(c) }

// Along reports whether a way or a Floor's band runs from from to its neighbour to, so a step
// between them goes along it rather than over the ground beside it.
func (r *BoardRead) Along(from, to cell.ID) bool {
	out, ok := grid.Link(r.Grid, from, to)
	if !ok {
		return false
	}
	back, _ := grid.Link(r.Grid, to, from)
	if r.Way(from).Links&out != 0 || r.Way(to).Links&back != 0 {
		return true
	}
	f, t := r.Built(from), r.Built(to)
	for k := range uint8(cell.Storeys) {
		if f, t := f.Storey(k), t.Storey(k); f.Allows != 0 && f.Links&out != 0 ||
			t.Allows != 0 && t.Links&back != 0 {
			return true
		}
	}
	return false
}

// CellVersion counts the changes to c alone; it only grows.
func (r *BoardRead) CellVersion(c cell.ID) uint64 { return r.cells.CellVersion(c) }

// Changes counts the changes to the board's cells: while it stays as it was, no cell has changed.
func (r *BoardRead) Changes() uint64 { return r.cells.Changes() }

// Version counts the changes to the terrain; it starts over with a load.
func (r *BoardRead) Version() uint64 { return r.cells.Version() }

// Passage counts the changes to what a step over the cells reads (who a cell admits and at what
// pace, its way, what is built over it, its heights): while it stays as it was, no cell's
// passage changed. It only grows.
func (r *BoardRead) Passage() uint64 { return r.cells.Passage() }

// CellPassage is the Passage at which c's passage last changed; it only grows.
func (r *BoardRead) CellPassage(c cell.ID) uint64 { return r.cells.CellPassage(c) }

// CellFingerprint is a number of what a step over c reads of it, its passage and the height
// under its middle, the same on every run for the same cell: a file kept of what was worked
// out over the board, a hierarchy of its routes, is good where its cells match.
func (r *BoardRead) CellFingerprint(c cell.ID) uint64 {
	f := r.cells.CellFingerprint(c)
	heights := r.Map().Heights()
	if heights == nil {
		return f
	}
	var buf [12]byte
	binary.LittleEndian.PutUint64(buf[:], f)
	binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(heights.At(r.CellCenter(c))))
	h := fnv.New64a()
	h.Write(buf[:])
	return h.Sum64()
}

// Map is what the board is drawn and priced by, its cells' kinds read through r: the plugin's,
// or the simple map on a board no plugin runs.
func (r *BoardRead) Map() MapRead { return readMap{r} }

func (r *BoardRead) altitude(c cell.ID) float32 {
	_, level := r.b.mapping.Top(c, 0) // the ground's level, whatever stands on it
	return level
}

// surfaces is where whoever moves over the board stands, the ground or a deck, read through r
type surfaces struct{ r *BoardRead }

var _ ground.Surfaces = surfaces{}

func (s surfaces) Surface(p geom.Vec, d cell.Domain, feet, step float32) (floor, ceiling float32) {
	if h := s.r.b.mapping.Heights(); h != nil {
		floor = h.At(p)
	}
	ceiling = f32.Inf(1)
	if d == 0 {
		d = cell.Land
	}
	sunk := f32.Inf(-1)
	s.r.DecksOver(p, func(c cell.ID, f cell.Floor) {
		deck := s.r.altitude(c) + f.Height
		if f.Sunk() {
			// under the ground whoever has its feet under the floor's roof stands on it
			deck = s.graded(c, f, p, deck)
			if roof := deck + f.Clear; f.Allows&d != 0 && deck <= feet+step && feet < roof &&
				deck > sunk {
				sunk, ceiling = deck, min(ceiling, roof)
			}
			return
		}
		if f.Allows&d != 0 && deck <= feet+step {
			floor = max(floor, deck)
		}
		if under := deck - f.Thickness; f.Thickness > 0 && under > feet {
			ceiling = min(ceiling, under)
		}
	})
	if !f32.IsInf(sunk, -1) {
		floor = sunk
	}
	return floor, ceiling
}

// graded is how high the sunk band f of c, its floor at level under c's middle, lies at p: on
// the way out to each neighbour its links name it runs to the mean of the two floors at their
// edge, so a tunnel climbs evenly from piece to piece
func (s surfaces) graded(c cell.ID, f cell.Floor, p geom.Vec, level float32) float32 {
	if f.Width <= 0 || f.Links == 0 {
		return level
	}
	r := s.r
	m := r.CellCenter(c)
	q := geom.NewVec(p.X-m.X, p.Y-m.Y)
	for way := range r.Directions() {
		if !f.Links.Has(way) {
			continue
		}
		st := r.Step(way)
		l := f32.Hypot(st.X, st.Y)
		along := (q.X*st.X + q.Y*st.Y) / l
		if along <= 0 {
			continue
		}
		n, ok := r.Toward(c, way)
		if !ok {
			return level
		}
		nb := r.Built(n)
		nf := nb.Storey(f.Storey)
		if !nf.Sunk() {
			return level
		}
		edge := (level + r.altitude(n) + nf.Height) / 2
		return level + (edge-level)*min(2*along/l, 1)
	}
	return level
}

// BoardWrite writes a board's terrain, on the game's goroutine.
type BoardWrite struct{ b *board }

// Set assigns c's terrain kind, taking effect immediately.
func (w *BoardWrite) Set(c cell.ID, kind cell.Kind) { w.b.cells.Set(c, kind) }

// SetAll resets every cell's terrain kind to kind.
func (w *BoardWrite) SetAll(kind cell.Kind) { w.b.cells.SetAll(kind) }

// SetWay lays w across c, the zero Way taking what ran there away; it takes effect immediately.
func (w *BoardWrite) SetWay(c cell.ID, way cell.Way) { w.b.cells.SetWay(c, way) }

// Touch counts a change to c made beyond the board, its heights shaped by a topography.
func (w *BoardWrite) Touch(c cell.ID) { w.b.cells.Touch(c) }
