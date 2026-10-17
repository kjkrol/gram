package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Free reports whether nothing solid fills the point (x, y, z): the ground, the kind standing on
// its cell to its Height, a deck's slab.
func (p *Plugin) Free(x, y, z float32) bool {
	at := geom.NewVec(float64(x), float64(y))
	if z < float32(p.topAt(at)) {
		return false
	}
	free := true
	p.boardPlugin.Board().DecksOver(at, func(c cell.ID, f cell.Floor) {
		deck := float32(p.relief.Altitude(c) + f.Height)
		if f.Height > 0 && z < deck && z >= deck-max(float32(f.Thickness), 1e-3) {
			free = false
		}
	})
	return free
}

// Span is the lowest and the highest the ground reaches.
func (p *Plugin) Span() (low, high float32) {
	l, h := p.relief.Extent()
	return float32(l), float32(h)
}

// Grain is how far a line of sight may step near a deck and how thin the thinnest one is; zero
// away from the decks.
func (p *Plugin) Grain(x, y float32) (step, thin float32) {
	brd := p.boardPlugin.Board()
	if !brd.NearDeck(geom.NewVec(float64(x), float64(y))) {
		return 0, 0
	}
	return float32(brd.NarrowestDeck() / 2), float32(brd.ThinnestDeck())
}
