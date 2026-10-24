package board

import (
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/render"
)

// drawnCells are the board's cells as the frame drawn reads them, as the last Publish copied
// them: each cell, by ordinal, its kind as crossed and bare and its way interned, the markers of
// the effects on it and one more than its version; the board's counts of changes they hold.
type drawnCells struct {
	cells            []drawnCell
	kinds            []cell.Kind
	ways             []cell.Way
	changes, version uint64
}

type drawnCell struct {
	kind, bare, way uint32
	states          tag.Tags[effect.States]
	ver             uint64
}

// drawn keeps the board's cells for drawing, each buffer brought up to the board as it is
// published: a cell is copied where its version differs from the one that buffer holds, so a
// buffer two frames old catches up. The kinds and the ways are interned, never changed once in.
type drawn struct {
	b       *board
	buf     render.Buffered[drawnCells]
	kinds   []cell.Kind
	kindOf  map[cell.Kind]uint32
	ways    []cell.Way
	wayOf   map[cell.Way]uint32
	back    *drawnCells
	visit   func(cell.ID) // copy, bound once
	swapped bool
}

func newDrawn(b *board) *drawn {
	d := &drawn{b: b, kindOf: map[cell.Kind]uint32{}, wayOf: map[cell.Way]uint32{}}
	d.visit = d.copy
	return d
}

// publish brings the back buffer up to the board.
func (d *drawn) publish() {
	bk, r := d.buf.Back(), &d.b.read
	bk.version = r.Version()
	if n := d.b.CellCount(); len(bk.cells) != n {
		bk.cells = make([]drawnCell, n)
	} else if bk.changes == r.Changes() {
		return
	}
	d.back = bk
	d.b.EachCell(d.visit)
	bk.changes, bk.kinds, bk.ways = r.Changes(), d.kinds, d.ways
}

func (d *drawn) swap() {
	d.buf.Swap()
	d.swapped = true
}

func (d *drawn) copy(c cell.ID) {
	i, ok := d.b.Ordinal(c)
	if !ok {
		return
	}
	r, rec := &d.b.read, &d.back.cells[i]
	v := r.CellVersion(c) + 1
	if rec.ver == v {
		return
	}
	rec.ver, rec.states = v, r.States(c)
	rec.kind, rec.bare, rec.way = d.kind(r.Kind(c)), d.kind(r.Bare(c)), d.way(r.Way(c))
}

func (d *drawn) kind(k cell.Kind) uint32 {
	i, ok := d.kindOf[k]
	if !ok {
		i = uint32(len(d.kinds))
		d.kinds = append(d.kinds, k)
		d.kindOf[k] = i
	}
	return i
}

func (d *drawn) way(w cell.Way) uint32 {
	i, ok := d.wayOf[w]
	if !ok {
		i = uint32(len(d.ways))
		d.ways = append(d.ways, w)
		d.wayOf[w] = i
	}
	return i
}

// front is the cells the frame drawn holds, nil before any was published
func (d *drawn) front() *drawnCells {
	if !d.swapped {
		return nil
	}
	return d.buf.Front()
}

// drawnBoard is the board as the frame drawn reads it (look.Board, draw.Ways): the cells as last
// published, the live board before any was.
type drawnBoard struct {
	grid.Grid
	d *drawn
}

var _ look.Board = drawnBoard{}

func (b drawnBoard) Shape() (grid.Shape, bool) { return grid.ShapeOf(b.Grid) }

// at is c's cell in f, false off the board
func (b drawnBoard) at(f *drawnCells, c cell.ID) (*drawnCell, bool) {
	i, ok := b.Ordinal(c)
	if !ok || i >= len(f.cells) {
		return nil, false
	}
	return &f.cells[i], true
}

func (b drawnBoard) Kind(c cell.ID) cell.Kind {
	if f := b.d.front(); f != nil {
		if rec, ok := b.at(f, c); ok {
			return f.kinds[rec.kind]
		}
		return cell.Kind{}
	}
	return b.d.b.read.Kind(c)
}

func (b drawnBoard) Bare(c cell.ID) cell.Kind {
	if f := b.d.front(); f != nil {
		if rec, ok := b.at(f, c); ok {
			return f.kinds[rec.bare]
		}
		return cell.Kind{}
	}
	return b.d.b.read.Bare(c)
}

func (b drawnBoard) Way(c cell.ID) cell.Way {
	if f := b.d.front(); f != nil {
		if rec, ok := b.at(f, c); ok {
			return f.ways[rec.way]
		}
		return cell.Way{}
	}
	return b.d.b.read.Way(c)
}

func (b drawnBoard) States(c cell.ID) tag.Tags[effect.States] {
	if f := b.d.front(); f != nil {
		if rec, ok := b.at(f, c); ok {
			return rec.states
		}
		return 0
	}
	return b.d.b.read.States(c)
}

func (b drawnBoard) CellVersion(c cell.ID) uint64 {
	if f := b.d.front(); f != nil {
		if rec, ok := b.at(f, c); ok && rec.ver > 0 {
			return rec.ver - 1
		}
		return 0
	}
	return b.d.b.read.CellVersion(c)
}

func (b drawnBoard) Changes() uint64 {
	if f := b.d.front(); f != nil {
		return f.changes
	}
	return b.d.b.read.Changes()
}

func (b drawnBoard) Version() uint64 {
	if f := b.d.front(); f != nil {
		return f.version
	}
	return b.d.b.read.Version()
}

// drawnMap is the board's Map as the frame drawn reads it: a cell's kind the drawn one
type drawnMap struct{ b drawnBoard }

var _ look.Map = drawnMap{}

func (m drawnMap) Look() look.Look         { return m.b.d.b.mapping.Look() }
func (m drawnMap) Heights() ground.Heights { return m.b.d.b.mapping.Heights() }
func (m drawnMap) Dressing() look.Dressing { return m.b.d.b.mapping.Dressing() }

func (m drawnMap) Top(c cell.ID) (corners [4]float32, level float32) {
	return m.b.d.b.mapping.Top(c, m.b.Kind(c).Height)
}
