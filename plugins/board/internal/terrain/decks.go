package terrain

import (
	"math/bits"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
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

// RailHalf is half how thick a railing along a band is on a grid of cells span wide.
func RailHalf(span float32) float32 { return span / 32 }

func banded(b *cell.Built) bool {
	return b != nil && b.Floor.Allows != 0 && b.Floor.Width > 0 && b.Floor.Links != 0
}

// through: the band runs on out the opposite way; else it begins in the cell's middle, a head
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

// q is seen from the cell's middle
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
		l := f32.Hypot(s.X, s.Y)
		ux, uy := s.X/l, s.Y/l
		along := q.X*ux + q.Y*uy
		if along >= 0 && along <= l/2 && f32.Abs(q.Y*ux-q.X*uy) <= half {
			return true
		}
	}
	return false
}

// BandArea is how much of r the band f runs as covers, the middle of its cell at m. The round of
// its middle is left out: a head's tile takes whoever comes onto the bridge.
func (rd *Read) BandArea(f cell.Floor, m geom.Vec, r geom.AABB) float32 {
	t := rd.t
	var area float32
	for i := range t.grid.Directions() {
		if !f.Links.Has(i) {
			continue
		}
		s := t.grid.Step(i)
		l := f32.Hypot(s.X, s.Y)
		area += stripArea(r, m, geom.NewVec(s.X/l, s.Y/l), l/2, f.Width/2)
	}
	return area
}

func stripArea(r geom.AABB, o, u geom.Vec, long, half float32) float32 {
	var in, out [8]geom.Vec
	in[0] = geom.NewVec(r.TopLeft.X-o.X, r.TopLeft.Y-o.Y)
	in[1] = geom.NewVec(r.BottomRight.X-o.X, r.TopLeft.Y-o.Y)
	in[2] = geom.NewVec(r.BottomRight.X-o.X, r.BottomRight.Y-o.Y)
	in[3] = geom.NewVec(r.TopLeft.X-o.X, r.BottomRight.Y-o.Y)
	n := 4
	for _, h := range [4]struct{ x, y, d float32 }{
		{-u.X, -u.Y, 0},
		{u.X, u.Y, long},
		{-u.Y, u.X, half},
		{u.Y, -u.X, half},
	} {
		k := 0
		for i := range n {
			p, q := in[i], in[(i+1)%n]
			dp, dq := p.X*h.x+p.Y*h.y-h.d, q.X*h.x+q.Y*h.y-h.d
			if dp <= 0 {
				out[k] = p
				k++
			}
			if dp < 0 && dq > 0 || dp > 0 && dq < 0 {
				t := dp / (dp - dq)
				out[k] = geom.NewVec(p.X+(q.X-p.X)*t, p.Y+(q.Y-p.Y)*t)
				k++
			}
		}
		in, n = out, k
		if n < 3 {
			return 0
		}
	}
	var twice float32
	for i := range n {
		p, q := in[i], in[(i+1)%n]
		twice += p.X*q.Y - q.X*p.Y
	}
	return f32.Abs(twice) / 2
}

// DecksOver calls visit with the floors over p in c, till it returns false: c's own — the whole
// cell, or a band p lies on — then those of the neighbours whose bands reach into c over p.
func (r *Read) DecksOver(c cell.ID, p geom.Vec, visit func(Deck) bool) {
	t := r.t
	if t.built == 0 || r.store == nil {
		return
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return
	}
	q := t.local(c, p)
	if b := r.store.builtOf(i); b != nil && b.Floor.Allows != 0 {
		if (!banded(b) || t.inBand(b.Floor, q)) && !visit(Deck{Cell: c, Built: *b}) {
			return
		}
	}
	if t.bands == 0 || !t.reached[i] {
		return
	}
	for j := range t.grid.Directions() {
		m, ok := t.grid.Toward(c, j)
		if !ok {
			continue
		}
		mi, _ := t.Ordinal(m)
		b := r.store.builtOf(mi)
		off := t.grid.Step(j)
		if banded(b) && t.inBand(b.Floor, geom.NewVec(q.X-off.X, q.Y-off.Y)) &&
			!visit(Deck{Cell: m, Built: *b, Off: off}) {
			return
		}
	}
}

// Bands appends to dst the decks running as bands on c and on its neighbours reaching into it.
func (r *Read) Bands(c cell.ID, dst []Deck) []Deck {
	t := r.t
	if t.bands == 0 || r.store == nil {
		return dst
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return dst
	}
	if b := r.store.builtOf(i); banded(b) {
		dst = append(dst, Deck{Cell: c, Built: *b})
	}
	if !t.reached[i] {
		return dst
	}
	for j := range t.grid.Directions() {
		if m, ok := t.grid.Toward(c, j); ok {
			mi, _ := t.Ordinal(m)
			if b := r.store.builtOf(mi); banded(b) {
				dst = append(dst, Deck{Cell: m, Built: *b, Off: t.grid.Step(j)})
			}
		}
	}
	return dst
}

// Ends are where a band deck on c, its middle at off, starts and ends along its axis, and whether
// each end stands free, no band running through carrying it on. It is false for a band that turns
// or forks.
func (r *Read) Ends(c cell.ID, f cell.Floor, off geom.Vec) (a, b geom.Vec, freeA, freeB, ok bool) {
	t := r.t
	d, through, ok := t.run(f)
	if !ok {
		return a, b, false, false, false
	}
	sd := t.grid.Step(d)
	b = geom.NewVec(off.X+sd.X/2, off.Y+sd.Y/2)
	if !through {
		return off, b, true, !r.carried(c, d), true
	}
	o := t.grid.Opposite(d)
	so := t.grid.Step(o)
	a = geom.NewVec(off.X+so.X/2, off.Y+so.Y/2)
	return a, b, !r.carried(c, o), !r.carried(c, d), true
}

// NearDeck reports whether a Floor lies over any of c: its own, or a neighbour's band reaching into
// it.
func (r *Read) NearDeck(c cell.ID) bool {
	t := r.t
	if t.built == 0 || r.store == nil {
		return false
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return false
	}
	if b := r.store.builtOf(i); b != nil && b.Floor.Allows != 0 {
		return true
	}
	return t.bands > 0 && t.reached[i]
}

// fencedBy reports whether a railing keeping d off on deck's level stands across the step from the
// origin to s: one on c's deck or on a neighbour's reaching into c, seen from c's middle at at
func (r *Read) fencedBy(c cell.ID, at, s geom.Vec, d cell.Domain, deck bool) bool {
	t := r.t
	i, ok := t.Ordinal(c)
	if !ok {
		return false
	}
	if r.railCrosses(c, r.store.builtOf(i), at, s, d, deck) {
		return true
	}
	if !t.reached[i] {
		return false
	}
	for j := range t.grid.Directions() {
		if m, ok := t.grid.Toward(c, j); ok {
			mi, _ := t.Ordinal(m)
			off := t.grid.Step(j)
			if r.railCrosses(m, r.store.builtOf(mi), geom.NewVec(at.X+off.X, at.Y+off.Y), s, d,
				deck) {
				return true
			}
		}
	}
	return false
}

// a head carries no railing, so whoever comes onto it comes from any side
func (r *Read) railCrosses(c cell.ID, b *cell.Built, off, s geom.Vec, d cell.Domain,
	deck bool) bool {
	if !banded(b) || b.Rail.Keeps&d == 0 || (b.Floor.Height > 0) != deck {
		return false
	}
	a, e, _, _, ok := r.Ends(c, b.Floor, off)
	if _, through, _ := r.t.run(b.Floor); !ok || !through {
		return false
	}
	l := f32.Hypot(e.X-a.X, e.Y-a.Y)
	half := b.Floor.Width / 2
	nx, ny := -(e.Y-a.Y)/l*half, (e.X-a.X)/l*half
	for _, side := range [2]float32{-1, 1} {
		ra := geom.NewVec(a.X+side*nx, a.Y+side*ny)
		rb := geom.NewVec(e.X+side*nx, e.Y+side*ny)
		if crosses(geom.Vec{}, s, ra, rb) {
			return true
		}
	}
	return false
}

func (r *Read) carried(c cell.ID, i int) bool {
	t := r.t
	n, ok := t.grid.Toward(c, i)
	if !ok {
		return false
	}
	ni, _ := t.Ordinal(n)
	b := r.store.builtOf(ni)
	if !banded(b) || !b.Floor.Links.Has(t.grid.Opposite(i)) {
		return false
	}
	_, through, _ := t.run(b.Floor)
	return through
}

// Fenced reports whether a railing keeping d off stands across the step from from to its neighbour
// to: one on a raised deck when the step is on the decks' level, deck, else one on the ground's.
func (r *Read) Fenced(from, to cell.ID, d cell.Domain, deck bool) bool {
	t := r.t
	if t.railed == 0 || r.store == nil {
		return false
	}
	way, ok := grid.Way(t.grid, from, to)
	if !ok {
		return false
	}
	s := t.grid.Step(way)
	return r.fencedBy(from, geom.Vec{}, s, d, deck) || r.fencedBy(to, s, s, d, deck)
}

// Railed reports whether a railing keeping d off runs along c's own band: one raised on the decks'
// level when deck, else one lying on the ground's; a head carries none.
func (r *Read) Railed(c cell.ID, d cell.Domain, deck bool) bool {
	t := r.t
	if t.railed == 0 || r.store == nil {
		return false
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return false
	}
	b := r.store.builtOf(i)
	if !banded(b) || b.Rail.Keeps&d == 0 || (b.Floor.Height > 0) != deck {
		return false
	}
	_, through, ok := t.run(b.Floor)
	return ok && through
}

// an end of the railing counts: where two railings join, each ends on the step
func crosses(p, q, a, b geom.Vec) bool {
	side := func(o, e, x geom.Vec) float32 { return (e.X-o.X)*(x.Y-o.Y) - (e.Y-o.Y)*(x.X-o.X) }
	return side(p, q, a)*side(p, q, b) <= 0 && side(a, b, p)*side(a, b, q) < 0
}

// p may lie in c anywhere round a wrapping square grid
func (t *Cells) local(c cell.ID, p geom.Vec) geom.Vec {
	if sq, ok := t.grid.(*grids.Square); ok {
		size := float32(sq.CellSize)
		x, y := f32.Floor(p.X/size), f32.Floor(p.Y/size)
		if in, ok := sq.Fold(int64(x), int64(y)); ok && in == c {
			return geom.NewVec(p.X-(x+0.5)*size, p.Y-(y+0.5)*size)
		}
	}
	m := t.grid.CellCenter(c)
	return geom.NewVec(p.X-m.X, p.Y-m.Y)
}

// Lane is how wide a body may be across the step from from to its neighbour to in d where a band
// runs along it: the narrower of the two less its railings keeping d off, +Inf where no band
// narrows the step. A raised band counts on the decks' level, deck, one lying on the ground on
// the ground's.
func (r *Read) Lane(from, to cell.ID, d cell.Domain, deck bool) float32 {
	t := r.t
	lane := f32.Inf(1)
	if t.bands == 0 || r.store == nil {
		return lane
	}
	for _, step := range [2][2]cell.ID{{from, to}, {to, from}} {
		b := r.Built(step[0])
		if !banded(&b) || b.Floor.Allows&d == 0 || (b.Floor.Height > 0) != deck ||
			!r.Reaches(step[0], step[1]) {
			continue
		}
		w := b.Floor.Width
		if b.Rail.Keeps&d != 0 {
			w -= 2 * RailHalf(t.grid.CellSpan())
		}
		lane = min(lane, w)
	}
	return lane
}

// Reaches reports whether c's deck runs out towards its neighbour n: a band along its link to n, a
// whole floor across any edge, and through a corner only along a link.
func (r *Read) Reaches(c, n cell.ID) bool {
	t := r.t
	b := r.Built(c)
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
