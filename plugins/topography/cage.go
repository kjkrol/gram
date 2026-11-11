package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Free reports whether nothing solid fills the point (x, y, z): the ground, but for the room
// over a floor sunk under it, the kind standing on its cell to its Height, a deck's slab.
func (r *Read) Free(x, y, z float32) bool {
	at := geom.NewVec(x, y)
	under := z < r.topAt(at)
	free := !under
	r.board().DecksOver(at, func(c cell.ID, f cell.Floor) {
		deck := r.p.relief.Altitude(c) + f.Height
		switch {
		case f.Sunk():
			if under && z >= deck && z < deck+f.Clear {
				free = true
			}
		case f.Height > 0 && z < deck && z >= deck-max(f.Thickness, 1e-3):
			free = false
		}
	})
	return free
}

// Span is the lowest and the highest the ground reaches.
func (r *Read) Span() (low, high float32) {
	l, h := r.p.relief.Extent()
	return l, h
}

// Grain is how far a line of sight may step near a deck and how thin the thinnest one is; zero
// away from the decks.
func (r *Read) Grain(x, y float32) (step, thin float32) {
	brd := r.board()
	if !brd.NearDeck(geom.NewVec(x, y)) {
		return 0, 0
	}
	return brd.NarrowestDeck() / 2, brd.ThinnestDeck()
}

func (r *Read) topAt(at geom.Vec) float32 {
	top := r.p.relief.GroundAt(at)
	brd := r.board()
	if c, ok := brd.CellAt(at); ok {
		top += brd.Kind(c).Height
	}
	return top
}
