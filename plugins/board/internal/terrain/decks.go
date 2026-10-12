package terrain

import (
	"math"
	"math/bits"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/grids"
)

// Deck is a floor over a point: the cell it is built on, what is built there, and that cell's
// middle seen from the middle of the cell asked about.
type Deck struct {
	Cell  cell.ID
	Built cell.Built
	Off   geom.Vec
}

// Railing is a railing along an edge of a deck's band, from A to B seen from the middle of the cell
// asked about, its ends free where the deck ends, the deck's cell and what is built there.
type Railing struct {
	Cell         cell.ID
	Built        cell.Built
	A, B         geom.Vec
	FreeA, FreeB bool
}

// banded reports whether b is a deck running as a band: a Width and Links to run along.
func banded(b *cell.Built) bool {
	return b != nil && b.Floor.Allows != 0 && b.Floor.Width > 0 && b.Floor.Links != 0
}

// run is the way a band runs out of its cell, d, and whether it runs on through and out the
// opposite way or begins in the cell's middle — a bridge's head; false for one that turns or forks.
func (t *Cells) run(f cell.Floor) (d int, through, ok bool) {
	d = bits.TrailingZeros8(uint8(f.Links))
	switch bits.OnesCount8(uint8(f.Links)) {
	case 1:
		return d, false, true
	case 2:
		return d, true, f.Links.Has(t.grid.Opposite(d))
	}
	return 0, false, false
}

// inBand reports whether q, seen from a cell's middle, lies on the band f runs as: on a piece from
// the middle out to the edge or the corner a link leaves by, Width wide, cut square at its end, or
// on the round of its middle.
func (t *Cells) inBand(f cell.Floor, q geom.Vec) bool {
	half := f.Width / 2
	if q.X*q.X+q.Y*q.Y <= half*half {
		return true
	}
	for i := range t.grid.Directions() {
		if !f.Links.Has(i) {
			continue
		}
		s := t.grid.Step(i)
		l := math.Hypot(s.X, s.Y)
		ux, uy := s.X/l, s.Y/l
		along := q.X*ux + q.Y*uy
		if along >= 0 && along <= l/2 && math.Abs(q.Y*ux-q.X*uy) <= half {
			return true
		}
	}
	return false
}

// DecksOver appends to dst the floors over p in c: c's own — the whole cell, or a band p lies on —
// and those of the neighbours whose bands reach into c over p.
func (t *Cells) DecksOver(c cell.ID, p geom.Vec, dst []Deck) []Deck {
	if t.built == 0 || t.store == nil {
		return dst
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return dst
	}
	q := t.local(c, p)
	if b := t.store.builtOf(i); b != nil && b.Floor.Allows != 0 {
		if !banded(b) || t.inBand(b.Floor, q) {
			dst = append(dst, Deck{Cell: c, Built: *b})
		}
	}
	if t.bands == 0 || !t.reached[i] {
		return dst
	}
	for j := range t.grid.Directions() {
		m, ok := t.grid.Toward(c, j)
		if !ok {
			continue
		}
		mi, _ := t.Ordinal(m)
		b := t.store.builtOf(mi)
		off := t.grid.Step(j)
		if banded(b) && t.inBand(b.Floor, geom.NewVec(q.X-off.X, q.Y-off.Y)) {
			dst = append(dst, Deck{Cell: m, Built: *b, Off: off})
		}
	}
	return dst
}

// Bands appends to dst the decks running as bands on c and on its neighbours reaching into it.
func (t *Cells) Bands(c cell.ID, dst []Deck) []Deck {
	if t.bands == 0 || t.store == nil {
		return dst
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return dst
	}
	if b := t.store.builtOf(i); banded(b) {
		dst = append(dst, Deck{Cell: c, Built: *b})
	}
	if !t.reached[i] {
		return dst
	}
	for j := range t.grid.Directions() {
		if m, ok := t.grid.Toward(c, j); ok {
			mi, _ := t.Ordinal(m)
			if b := t.store.builtOf(mi); banded(b) {
				dst = append(dst, Deck{Cell: m, Built: *b, Off: t.grid.Step(j)})
			}
		}
	}
	return dst
}

// Ends are where a band deck on c, its middle at off, starts and ends along its axis — a head's
// in the cell's middle — and whether each end stands free, where no band running through carries
// it on; false for a band that turns or forks.
func (t *Cells) Ends(c cell.ID, f cell.Floor, off geom.Vec) (a, b geom.Vec, freeA, freeB, ok bool) {
	d, through, ok := t.run(f)
	if !ok {
		return a, b, false, false, false
	}
	sd := t.grid.Step(d)
	b = geom.NewVec(off.X+sd.X/2, off.Y+sd.Y/2)
	if !through {
		return off, b, true, !t.carried(c, d), true
	}
	o := t.grid.Opposite(d)
	so := t.grid.Step(o)
	a = geom.NewVec(off.X+so.X/2, off.Y+so.Y/2)
	return a, b, !t.carried(c, o), !t.carried(c, d), true
}

// Banded reports whether a deck runs as a band on any cell.
func (t *Cells) Banded() bool { return t.bands > 0 }

// Narrowest is how wide the narrowest band is; 0 for none.
func (t *Cells) Narrowest() float64 { return t.narrowest }

// Thinnest is how thick the thinnest raised band's slab is; 0 for none.
func (t *Cells) Thinnest() float64 { return t.thinnest }

// NearDeck reports whether a Floor lies over any of c: its own, or a neighbour's band reaching into
// it.
func (t *Cells) NearDeck(c cell.ID) bool {
	if t.built == 0 || t.store == nil {
		return false
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return false
	}
	if b := t.store.builtOf(i); b != nil && b.Floor.Allows != 0 {
		return true
	}
	return t.bands > 0 && t.reached[i]
}

// Railings appends to dst the railings standing on c's deck and on its neighbours' reaching into
// c, seen from c's middle.
func (t *Cells) Railings(c cell.ID, dst []Railing) []Railing {
	if t.railed == 0 || t.store == nil {
		return dst
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return dst
	}
	dst = t.railingsOf(c, t.store.builtOf(i), geom.Vec{}, dst)
	if !t.reached[i] {
		return dst
	}
	for j := range t.grid.Directions() {
		if m, ok := t.grid.Toward(c, j); ok {
			mi, _ := t.Ordinal(m)
			dst = t.railingsOf(m, t.store.builtOf(mi), t.grid.Step(j), dst)
		}
	}
	return dst
}

// railingsOf appends to dst the two railings along the edges of the band b runs as on c, c's
// middle at off; a head carries none, so whoever comes onto it comes from any side.
func (t *Cells) railingsOf(c cell.ID, b *cell.Built, off geom.Vec, dst []Railing) []Railing {
	if !banded(b) || b.Rail.Keeps == 0 {
		return dst
	}
	a, e, freeA, freeB, ok := t.Ends(c, b.Floor, off)
	if _, through, _ := t.run(b.Floor); !ok || !through {
		return dst
	}
	l := math.Hypot(e.X-a.X, e.Y-a.Y)
	half := b.Floor.Width / 2
	nx, ny := -(e.Y-a.Y)/l*half, (e.X-a.X)/l*half
	for _, side := range [2]float64{-1, 1} {
		dst = append(dst, Railing{
			Cell:  c,
			Built: *b,
			A:     geom.NewVec(a.X+side*nx, a.Y+side*ny),
			B:     geom.NewVec(e.X+side*nx, e.Y+side*ny),
			FreeA: freeA,
			FreeB: freeB,
		})
	}
	return dst
}

// carried reports whether c's band runs on the way i into a neighbour whose band comes back and
// runs on through: a head carries nothing on.
func (t *Cells) carried(c cell.ID, i int) bool {
	n, ok := t.grid.Toward(c, i)
	if !ok {
		return false
	}
	ni, _ := t.Ordinal(n)
	b := t.store.builtOf(ni)
	if !banded(b) || !b.Floor.Links.Has(t.grid.Opposite(i)) {
		return false
	}
	_, through, _ := t.run(b.Floor)
	return through
}

// Fenced reports whether a railing keeping d off stands across the step from from to its neighbour
// to: one on a raised deck when the step is on the decks' level, deck, else one on the ground's.
func (t *Cells) Fenced(from, to cell.ID, d cell.Domain, deck bool) bool {
	if t.railed == 0 || t.store == nil {
		return false
	}
	way, ok := grid.Way(t.grid, from, to)
	if !ok {
		return false
	}
	s := t.grid.Step(way)
	t.rails = t.Railings(from, t.rails[:0])
	near := len(t.rails)
	t.rails = t.Railings(to, t.rails)
	for k, r := range t.rails {
		if r.Built.Rail.Keeps&d == 0 || (r.Built.Floor.Height > 0) != deck {
			continue
		}
		a, b := r.A, r.B
		if k >= near { // seen from to's middle
			a, b = geom.NewVec(a.X+s.X, a.Y+s.Y), geom.NewVec(b.X+s.X, b.Y+s.Y)
		}
		if crosses(geom.Vec{}, s, a, b) {
			return true
		}
	}
	return false
}

// crosses reports whether the step p–q crosses the railing a–b: from one side of it to the other,
// over it or an end of it — where two railings join, each ends on the step.
func crosses(p, q, a, b geom.Vec) bool {
	side := func(o, e, x geom.Vec) float64 { return (e.X-o.X)*(x.Y-o.Y) - (e.Y-o.Y)*(x.X-o.X) }
	return side(p, q, a)*side(p, q, b) <= 0 && side(a, b, p)*side(a, b, q) < 0
}

// local is p seen from c's middle, wherever round a wrapping square grid p lies in c.
func (t *Cells) local(c cell.ID, p geom.Vec) geom.Vec {
	if sq, ok := t.grid.(*grids.Square); ok {
		size := float64(sq.CellSize)
		x, y := math.Floor(p.X/size), math.Floor(p.Y/size)
		if in, ok := sq.Fold(int64(x), int64(y)); ok && in == c {
			return geom.NewVec(p.X-(x+0.5)*size, p.Y-(y+0.5)*size)
		}
	}
	m := t.grid.CellCenter(c)
	return geom.NewVec(p.X-m.X, p.Y-m.Y)
}

// Reaches reports whether c's deck runs out towards its neighbour n: a band along its link to n, a
// whole floor across any edge, and through a corner only along a link.
func (t *Cells) Reaches(c, n cell.ID) bool {
	b := t.Built(c)
	if b.Floor.Allows == 0 {
		return false
	}
	for i := range t.grid.Directions() {
		if m, ok := t.grid.Toward(c, i); ok && m == n {
			if banded(&b) || t.grid.Corner(i) {
				return b.Floor.Links.Has(i)
			}
			return true
		}
	}
	return false
}

// decked counts the bands of sites and marks the cells a neighbour's band reaches into — the flanks
// of a band leaving through a corner.
func (t *Cells) decked(sites []site) {
	t.bands, t.narrowest, t.thinnest = 0, 0, 0
	for _, s := range sites {
		b := s.built
		if !banded(&b) {
			continue
		}
		if t.bands == 0 {
			if len(t.reached) != t.grid.CellCount() {
				t.reached = make([]bool, t.grid.CellCount())
			}
			clear(t.reached)
		}
		t.bands++
		if t.narrowest == 0 || b.Floor.Width < t.narrowest {
			t.narrowest = b.Floor.Width
		}
		if f := b.Floor; f.Height > 0 && f.Thickness > 0 && (t.thinnest == 0 ||
			f.Thickness < t.thinnest) {
			t.thinnest = f.Thickness
		}
		for i := range t.grid.Directions() {
			fa, fb, corner := t.grid.Flanks(i)
			if !corner || !b.Floor.Links.Has(i) {
				continue
			}
			for _, f := range [2]int{fa, fb} {
				if n, ok := t.grid.Toward(s.cell, f); ok {
					if ni, ok := t.Ordinal(n); ok {
						t.reached[ni] = true
					}
				}
			}
		}
	}
}
