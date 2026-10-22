package terrain

import (
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Read reads the cells' terrain through queries of its own, for one goroutine at a time; what
// the cells share it reads as they stand.
type Read struct {
	t     *Cells
	store *store // nil before the cells are made: the seed is read
}

// Read is the cells' own Read, on the queries they were made with.
func (t *Cells) Read() *Read { return &t.read }

// Fork is a Read of the same cells through queries of its own, made anew whenever the cells are:
// it reads the seed till then.
func (r *Read) Fork() *Read {
	f := &Read{t: r.t}
	if r.t.store != nil {
		f.store = r.t.store.fork()
	}
	r.t.forks = append(r.t.forks, f)
	return f
}

// Ordinal is the grid's, straight from the id on a square grid.
func (r *Read) Ordinal(c cell.ID) (int, bool) { return r.t.Ordinal(c) }

// CellVersion counts the changes to c; it only grows, and changes to other cells leave it as it is.
func (r *Read) CellVersion(c cell.ID) uint64 { return r.t.CellVersion(c) }

// Changes counts the changes to the cells, one at a time or all at once.
func (r *Read) Changes() uint64 { return r.t.changes }

// Version counts the changes to the terrain, written or by an effect; it starts over with a load.
func (r *Read) Version() uint64 { return r.t.Version() }

// Banded reports whether a deck runs as a band on any cell.
func (r *Read) Banded() bool { return r.t.bands > 0 }

// Narrowest is how wide the narrowest band is; 0 for none.
func (r *Read) Narrowest() float64 { return r.t.narrowest }

// Thinnest is how thick the thinnest raised band's slab is; 0 for none.
func (r *Read) Thinnest() float64 { return r.t.thinnest }
