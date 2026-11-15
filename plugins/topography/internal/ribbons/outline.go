package ribbons

import (
	"cmp"
	"maps"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/topography/painter"
)

// face is roadway as it is drawn: a triangle of a piece the shader draws as two, or a plate,
// its outline running round its inside on the left, a plate's as its circle; the triangles
// its heights are worked out by, and the piece it is
type face struct {
	tri                        [2][3]geom.Vec
	tz                         [2][3]float32
	poly                       []geom.Vec
	disc                       bool
	mid                        geom.Vec
	radius                     float32
	lo, hi                     geom.Vec
	sector, index, owner, part int
}

// holds says q lies in f by margin at least
func (f *face) holds(q geom.Vec, margin float32) bool {
	if q.X < f.lo.X || q.X > f.hi.X || q.Y < f.lo.Y || q.Y > f.hi.Y {
		return false
	}
	if f.disc {
		return gap(q, f.mid) <= f.radius-margin
	}
	for i, a := range f.poly {
		b := f.poly[(i+1)%len(f.poly)]
		d := geom.NewVec(b.X-a.X, b.Y-a.Y)
		l := f32.Hypot(d.X, d.Y)
		if l == 0 {
			continue
		}
		if (d.X*(q.Y-a.Y)-d.Y*(q.X-a.X))/l < margin {
			return false
		}
	}
	return true
}

// zAt is how high f lies at q, as the shader draws it: from the triangle q lies in, else the
// nearest
func (f *face) zAt(q geom.Vec) float32 {
	best, out := f.tz[0][0], float32(math.MaxFloat32)
	for t := range 2 {
		a, b, c := f.tri[t][0], f.tri[t][1], f.tri[t][2]
		det := (b.Y-c.Y)*(a.X-c.X) + (c.X-b.X)*(a.Y-c.Y)
		if f32.Abs(det) < 1e-6 {
			continue
		}
		u := ((b.Y-c.Y)*(q.X-c.X) + (c.X-b.X)*(q.Y-c.Y)) / det
		v := ((c.Y-a.Y)*(q.X-c.X) + (a.X-c.X)*(q.Y-c.Y)) / det
		w := 1 - u - v
		z := u*f.tz[t][0] + v*f.tz[t][1] + w*f.tz[t][2]
		miss := max(-u, -v, -w, 0)
		if miss == 0 {
			return z
		}
		if miss < out {
			best, out = z, miss
		}
	}
	return best
}

// facesOf are the faces of the piece p: its two triangles, or its plate
func facesOf(p []float32) []face {
	p0, p1 := geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3])
	n0, n1 := geom.NewVec(p[4], p[5]), geom.NewVec(p[6], p[7])
	a0, a1 := geom.NewVec(p0.X-n0.X, p0.Y-n0.Y), geom.NewVec(p0.X+n0.X, p0.Y+n0.Y)
	b0, b1 := geom.NewVec(p1.X-n1.X, p1.Y-n1.Y), geom.NewVec(p1.X+n1.X, p1.Y+n1.Y)
	tri := [2][3]geom.Vec{{a0, b0, a1}, {b0, b1, a1}}
	tz := [2][3]float32{{p[8], p[9], p[8]}, {p[9], p[9], p[8]}}
	bound := func(f *face) {
		f.lo, f.hi = f.poly[0], f.poly[0]
		for _, c := range f.poly {
			f.lo = geom.NewVec(min(f.lo.X, c.X), min(f.lo.Y, c.Y))
			f.hi = geom.NewVec(max(f.hi.X, c.X), max(f.hi.Y, c.Y))
		}
	}
	if int(p[13])&disc != 0 {
		f := face{tri: tri, tz: tz, disc: true}
		f.mid = geom.NewVec((p0.X+p1.X)/2, (p0.Y+p1.Y)/2)
		f.radius = gap(p0, p1) / 2
		if f.radius <= 0 {
			return nil
		}
		for k := range sides {
			sin, cos := f32.Sincos(2 * math.Pi * float32(k) / sides)
			f.poly = append(f.poly, geom.NewVec(f.mid.X+cos*f.radius, f.mid.Y+sin*f.radius))
		}
		bound(&f)
		return []face{f}
	}
	var out []face
	for t := range 2 {
		at := []geom.Vec{tri[t][0], tri[t][1], tri[t][2]}
		a := area(at)
		if f32.Abs(a) < 1e-4 {
			continue
		}
		f := face{tz: [2][3]float32{tz[t], tz[t]}, poly: at}
		f.tri = [2][3]geom.Vec{tri[t], tri[t]}
		if a < 0 {
			f.poly = []geom.Vec{at[0], at[2], at[1]}
		}
		bound(&f)
		out = append(out, f)
	}
	return out
}

// area is the signed area of the polygon at: positive where it runs round its inside on the
// left. Summed off its first point and in float64: a small polygon far off the board's corner
// has its products of coordinates lose its area in float32.
func area(at []geom.Vec) float32 {
	if len(at) == 0 {
		return 0
	}
	ox, oy := float64(at[0].X), float64(at[0].Y)
	s := 0.0
	for i, a := range at {
		b := at[(i+1)%len(at)]
		ax, ay := float64(a.X)-ox, float64(a.Y)-oy
		bx, by := float64(b.X)-ox, float64(b.Y)-oy
		s += ax*by - bx*ay
	}
	return float32(s / 2)
}

// the sides a plate's circle is taken as; metres: the squares the faces are looked up by, how
// far off an edge it is tried by, how near two ends of edges are one place, how high two
// pieces of roads not meeting may lie apart where they overlap to be one roadway, the radius
// a corner of the roadway is rounded by and the most a rounding runs along either edge; the
// least turn of the roadway's edge rounded, degrees, and how near turns of it are to round
// them as one, metres; how far off the edge a step of it may be taken out, how wide across
// its mouth and how long along the edge a notch filled is, metres, and how big, square
// metres; how far the ground may rise over the road where a corner is rounded or a notch or
// a hole filled, metres; the most a hole in the roadway filled is, square metres, and the
// most a slot of it filled, however long, is across, metres
const (
	sides     = 48
	plot      = 8
	probe     = 0.02
	weld      = 0.06
	apartBy   = 1.5
	filletR   = 4
	filletMax = 7
	filletMin = 20
	group     = 1
	smooth    = 0.15
	notchW    = 6
	notchL    = 14
	notchA    = 25
	cutBy     = 0.3
	holeMin   = 10
	holeMax   = 40
	slot      = 1.2
)

// grid is the faces by the squares they lie in, and the roadway laid round them since: the
// corners rounded, the holes and notches filled
type grid struct {
	faces []face
	at    map[[2]int][]int
	stamp []int
	tag   int
	side  float32
	laid  map[[2]int][]face
}

// lay notes the triangle at, as high as h, as roadway laid
func (g *grid) lay(at [3]geom.Vec, h [3]float32) {
	f := face{tri: [2][3]geom.Vec{at, at}, tz: [2][3]float32{h, h}, poly: at[:]}
	if area(f.poly) < 0 {
		f.poly = []geom.Vec{at[0], at[2], at[1]}
	}
	f.lo, f.hi = at[0], at[0]
	for _, c := range at {
		f.lo = geom.NewVec(min(f.lo.X, c.X), min(f.lo.Y, c.Y))
		f.hi = geom.NewVec(max(f.hi.X, c.X), max(f.hi.Y, c.Y))
	}
	a, b := g.key(f.lo.X, f.lo.Y), g.key(f.hi.X, f.hi.Y)
	for x := a[0]; x <= b[0]; x++ {
		for y := a[1]; y <= b[1]; y++ {
			g.laid[[2]int{x, y}] = append(g.laid[[2]int{x, y}], f)
		}
	}
}

// paved says roadway lies at q, of the faces or laid since, one with the road of owner lying
// at z there
func (r *Renderer) paved(g *grid, q geom.Vec, z float32, owner int) bool {
	k := g.key(q.X, q.Y)
	for _, i := range g.at[k] {
		if f := &g.faces[i]; f.holds(q, 0) && r.one(owner, f.owner, f32.Abs(f.zAt(q)-z)) {
			return true
		}
	}
	for i := range g.laid[k] {
		if f := &g.laid[k][i]; f.holds(q, 0) && f32.Abs(f.zAt(q)-z) < 2*apartBy*r.cfg.Metre {
			return true
		}
	}
	return false
}

func (g *grid) key(x, y float32) [2]int {
	return [2]int{int(f32.Floor(x / g.side)), int(f32.Floor(y / g.side))}
}

// near is the faces whose bounds reach the box from lo to hi, but self
func (g *grid) near(lo, hi geom.Vec, self int, into []int) []int {
	g.tag++
	into = into[:0]
	a, b := g.key(lo.X, lo.Y), g.key(hi.X, hi.Y)
	for x := a[0]; x <= b[0]; x++ {
		for y := a[1]; y <= b[1]; y++ {
			for _, k := range g.at[[2]int{x, y}] {
				f := &g.faces[k]
				if k == self || g.stamp[k] == g.tag || f.hi.X < lo.X || f.lo.X > hi.X ||
					f.hi.Y < lo.Y || f.lo.Y > hi.Y {
					continue
				}
				g.stamp[k] = g.tag
				into = append(into, k)
			}
		}
	}
	return into
}

// over is the highest the roadway of owner, lying at z, lies at q
func (r *Renderer) over(g *grid, q geom.Vec, z float32, owner int) float32 {
	top := z
	key := g.key(q.X, q.Y)
	for _, k := range g.at[key] {
		f := &g.faces[k]
		if !f.holds(q, 0) {
			continue
		}
		if h := f.zAt(q); r.one(owner, f.owner, f32.Abs(h-z)) {
			top = max(top, h)
		}
	}
	for i := range g.laid[key] {
		f := &g.laid[key][i]
		if !f.holds(q, 0) {
			continue
		}
		if h := f.zAt(q); f32.Abs(h-z) < 2*apartBy*r.cfg.Metre {
			top = max(top, h)
		}
	}
	return top
}

// brink is a stretch of the roadway's edge, the roadway on its left from a to b, with the
// heights there and the face it is of
type brink struct {
	a, b   geom.Vec
	za, zb float32
	face   int
}

// chain is the roadway's edge as one runs along it, the roadway on the left: its points, how
// high each is, the face each runs on along, and whether it closes
type chain struct {
	at     []geom.Vec
	z      []float32
	face   []int
	closed bool
}

// begun is a closed chain begun halfway along its longest stretch: where it begins and ends
// it neither turns nor is simplified away, so a corner there is rounded as any
func begun(ch chain) chain {
	n := len(ch.at)
	if !ch.closed || n < 3 {
		return ch
	}
	at, long := 0, float32(-1)
	for i := range n {
		if l := gap(ch.at[i], ch.at[(i+1)%n]); l > long {
			at, long = i, l
		}
	}
	j := (at + 1) % n
	mid := geom.NewVec((ch.at[at].X+ch.at[j].X)/2, (ch.at[at].Y+ch.at[j].Y)/2)
	out := chain{closed: true}
	out.at = append(out.at, mid)
	out.z = append(out.z, (ch.z[at]+ch.z[j])/2)
	out.face = append(out.face, ch.face[at])
	for k := range n {
		i := (j + k) % n
		out.at = append(out.at, ch.at[i])
		out.z = append(out.z, ch.z[i])
		out.face = append(out.face, ch.face[i])
	}
	return out
}

// outline lays the roadway's edge whole: every piece of every road with its lanes taken as one
// roadway, a hole in it smaller than a road's corner filled, each corner the roadway turns in
// rounded, and a line along its edge wherever it ends; a road's lanes are marked only where no
// other road lies on it, and a roundabout's rim is broken across where roads run in
func (r *Renderer) outline(ground func(geom.Vec) float32) {
	m := r.cfg.Metre
	r.ground = ground
	g := r.gather()
	brinks := r.brinks(g)
	chains := r.chains(brinks)
	into, cur, part, fine := r.into, r.cur, r.part, r.fine
	r.into, r.fine, r.part = r.pieces, true, -1
	var kept []chain
	for _, ch := range chains {
		if ch.closed && area(ch.at) < 0 && small(ch.at, m) && sunk(ch.at, ch.z, ground, m) {
			r.fill(&g, ch, ground)
			continue
		}
		ch = simplify(begun(ch), smooth*m, 0.02*m)
		ch = r.close(&g, ch)
		kept = append(kept, r.fillet(&g, ch, ground))
	}
	for _, ch := range kept {
		r.line(&g, ch)
		r.bank(&g, ch, ground)
	}
	r.rims(&g, kept)
	r.marks(&g)
	r.unbank(&g)
	r.onTop()
	r.into, r.cur, r.part, r.fine = into, cur, part, fine
}

// gather is the faces of the roadway: the pieces of the roads with lanes, on the ground or
// a deck, no island, bank or line
func (r *Renderer) gather() grid {
	m := r.cfg.Metre
	g := grid{at: map[[2]int][]int{}, side: plot * m, laid: map[[2]int][]face{}}
	for _, sector := range slices.Sorted(maps.Keys(r.pieces)) {
		all := r.pieces[sector]
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			kind := int(p[13])
			if kind&(bank|island|marking) != 0 || p[15] != 0 {
				continue
			}
			owner := r.owners[sector][i/floats]
			if r.nodes[owner].lanes == 0 || [3]float32(p[16:19]) == lineRGB {
				continue
			}
			for _, f := range facesOf(p) {
				f.sector, f.index, f.owner = sector, i/floats, owner
				f.part = r.parts[sector][i/floats]
				a, b := g.key(f.lo.X, f.lo.Y), g.key(f.hi.X, f.hi.Y)
				for x := a[0]; x <= b[0]; x++ {
					for y := a[1]; y <= b[1]; y++ {
						g.at[[2]int{x, y}] = append(g.at[[2]int{x, y}], len(g.faces))
					}
				}
				g.faces = append(g.faces, f)
			}
		}
	}
	g.stamp = make([]int, len(g.faces))
	return g
}

// brinks are the stretches of the faces' outlines no other face lies beyond: each outline cut
// where another's crosses it, a stretch kept where neither its middle lies in another face nor
// the roadway runs on just past it. Where two outlines cross is worked out once, the same for
// both, so the stretches either side meet at one place however sharp the crossing.
func (r *Renderer) brinks(g grid) []brink {
	m := r.cfg.Metre
	var out []brink
	var cand []int
	type cut struct {
		t  float32
		at geom.Vec
	}
	var cuts []cut
	for fi := range g.faces {
		f := &g.faces[fi]
		cand = g.near(geom.NewVec(f.lo.X-m, f.lo.Y-m), geom.NewVec(f.hi.X+m, f.hi.Y+m), fi, cand)
		for e, a := range f.poly {
			b := f.poly[(e+1)%len(f.poly)]
			d := geom.NewVec(b.X-a.X, b.Y-a.Y)
			long := f32.Hypot(d.X, d.Y)
			if long < 0.01*m {
				continue
			}
			// the roadway lies on the left: away is the right
			away := geom.NewVec(d.Y/long, -d.X/long)
			cuts = append(cuts[:0], cut{0, a}, cut{1, b})
			for _, k := range cand {
				o := &g.faces[k]
				for i, c := range o.poly {
					c2 := o.poly[(i+1)%len(o.poly)]
					x, ok := crossAt(fi, e, a, b, k, i, c, c2)
					if !ok {
						continue
					}
					t := ((x.X-a.X)*d.X + (x.Y-a.Y)*d.Y) / (long * long)
					if t > 0 && t < 1 {
						cuts = append(cuts, cut{t, x})
					}
				}
			}
			slices.SortFunc(cuts, func(x, y cut) int { return cmp.Compare(x.t, y.t) })
			for c := 1; c < len(cuts); c++ {
				t0, t1 := cuts[c-1].t, cuts[c].t
				if (t1-t0)*long < 0.005*m {
					continue
				}
				mid := geom.NewVec(a.X+d.X*(t0+t1)/2, a.Y+d.Y*(t0+t1)/2)
				pr := geom.NewVec(mid.X+away.X*probe*m, mid.Y+away.Y*probe*m)
				if r.off != nil && r.off(pr) {
					continue
				}
				zm := f.zAt(mid)
				inside := false
				for _, k := range cand {
					o := &g.faces[k]
					if o.holds(pr, 0) && r.one(f.owner, o.owner, f32.Abs(o.zAt(pr)-zm)) ||
						o.holds(mid, 0.001*m) &&
							r.one(f.owner, o.owner, f32.Abs(o.zAt(mid)-zm)) {
						inside = true
						break
					}
				}
				if inside {
					continue
				}
				pa, pb := cuts[c-1].at, cuts[c].at
				out = append(out, brink{a: pa, b: pb, za: f.zAt(pa), zb: f.zAt(pb), face: fi})
			}
		}
	}
	return out
}

// one says the faces f and o, lying apart by so much where they meet, are one roadway: about
// as high, or of one road or two meeting, which on a hillside lie apart where they overlap
func (r *Renderer) one(f, o int, apart float32) bool {
	if apart < apartBy*r.cfg.Metre || f == o {
		return true
	}
	a, b := &r.nodes[f], &r.nodes[o]
	if a.deck != b.deck && !a.head && !b.head {
		return false
	}
	if slices.Contains(a.arms, o) {
		return true
	}
	for _, k := range a.arms {
		if slices.Contains(b.arms, k) {
			return true
		}
	}
	return false
}

// crossAt is where the edge e of the face f, from a to b, and the edge i of the face k, from c
// to d, cross: worked out from the edge of the lower face, the same whichever is asked
func crossAt(f, e int, a, b geom.Vec, k, i int, c, d geom.Vec) (geom.Vec, bool) {
	if k < f || k == f && i < e {
		a, b, c, d = c, d, a, b
	}
	d1 := geom.NewVec(b.X-a.X, b.Y-a.Y)
	d2 := geom.NewVec(d.X-c.X, d.Y-c.Y)
	den := d1.X*d2.Y - d1.Y*d2.X
	l1, l2 := d1.X*d1.X+d1.Y*d1.Y, d2.X*d2.X+d2.Y*d2.Y
	if den*den < 1e-12*l1*l2 {
		return geom.Vec{}, false
	}
	t := ((c.X-a.X)*d2.Y - (c.Y-a.Y)*d2.X) / den
	u := ((c.X-a.X)*d1.Y - (c.Y-a.Y)*d1.X) / den
	if t < 0 || t > 1 || u < 0 || u > 1 {
		return geom.Vec{}, false
	}
	return geom.NewVec(a.X+d1.X*t, a.Y+d1.Y*t), true
}

// chains strings the brinks into the edges they make: ends nearer than weld are one place, a
// brink twice is one, a brink both ways is none; where more than one runs on from a place,
// the one turning most to the left, which keeps to the roadway it came along
func (r *Renderer) chains(brinks []brink) []chain {
	m := r.cfg.Metre
	var places []geom.Vec
	spots := map[[2]int][]int{}
	place := func(q geom.Vec) int {
		cx, cy := int(f32.Floor(q.X/(weld*m))), int(f32.Floor(q.Y/(weld*m)))
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for _, v := range spots[[2]int{cx + dx, cy + dy}] {
					if gap(places[v], q) < weld*m {
						return v
					}
				}
			}
		}
		places = append(places, q)
		spots[[2]int{cx, cy}] = append(spots[[2]int{cx, cy}], len(places)-1)
		return len(places) - 1
	}
	va, vb := make([]int, len(brinks)), make([]int, len(brinks))
	for i, k := range brinks {
		va[i], vb[i] = place(k.a), place(k.b)
	}
	seen := map[[2]int]bool{}
	keep := make([]bool, len(brinks))
	for i := range brinks {
		e := [2]int{va[i], vb[i]}
		if va[i] == vb[i] || seen[e] {
			continue
		}
		seen[e], keep[i] = true, true
	}
	out := make([][]int, len(places))
	in := make([][]int, len(places))
	for i := range brinks {
		if keep[i] && seen[[2]int{vb[i], va[i]}] {
			keep[i] = false
		}
		if keep[i] {
			out[va[i]] = append(out[va[i]], i)
			in[vb[i]] = append(in[vb[i]], i)
		}
	}
	used := make([]bool, len(brinks))
	follow := func(first int) chain {
		var ch chain
		ch.at = append(ch.at, places[va[first]])
		ch.z = append(ch.z, brinks[first].za)
		start := va[first]
		at := first
		for {
			used[at] = true
			k := &brinks[at]
			ch.face = append(ch.face, k.face)
			v := vb[at]
			if v == start {
				ch.closed = true
				return ch
			}
			ch.at = append(ch.at, places[v])
			ch.z = append(ch.z, k.zb)
			way := geom.NewVec(k.b.X-k.a.X, k.b.Y-k.a.Y)
			next, best := -1, float32(-10)
			for _, j := range out[v] {
				if used[j] {
					continue
				}
				o := &brinks[j]
				to := geom.NewVec(o.b.X-o.a.X, o.b.Y-o.a.Y)
				turn := f32.Atan2(way.X*to.Y-way.Y*to.X, way.X*to.X+way.Y*to.Y)
				if turn > best {
					next, best = j, turn
				}
			}
			if next < 0 {
				// an end next to where it began: the same place, by a hair the weld missed
				if len(ch.at) > 2 && gap(places[v], places[start]) < 4*weld*m {
					ch.at, ch.z = ch.at[:len(ch.at)-1], ch.z[:len(ch.z)-1]
					ch.closed = true
					return ch
				}
				ch.face = append(ch.face, k.face)
				return ch
			}
			at = next
		}
	}
	// a chain is followed from where it begins: back along it while one way leads in
	begin := func(i int) int {
		at := i
		for range len(brinks) {
			prev := -1
			for _, j := range in[va[at]] {
				if used[j] {
					continue
				}
				if prev >= 0 {
					return at
				}
				prev = j
			}
			if prev < 0 || prev == i {
				return at
			}
			at = prev
		}
		return at
	}
	var chains []chain
	for i := range brinks {
		if keep[i] && !used[i] && len(in[va[i]]) == 0 {
			chains = append(chains, follow(i))
		}
	}
	for i := range brinks {
		if keep[i] && !used[i] {
			chains = append(chains, follow(begin(i)))
		}
	}
	// an edge followed to where another was begun runs on into it; one run on into itself
	// closes
	for {
		joined := false
		for a := range chains {
			ca := &chains[a]
			if ca.closed || len(ca.at) == 0 {
				continue
			}
			end := ca.at[len(ca.at)-1]
			if gap(end, ca.at[0]) < 3*weld*m && len(ca.at) > 2 {
				ca.at, ca.z = ca.at[:len(ca.at)-1], ca.z[:len(ca.z)-1]
				ca.face = ca.face[:len(ca.face)-1]
				ca.closed, joined = true, true
				continue
			}
			best, near := -1, 3*weld*m
			for b := range chains {
				cb := &chains[b]
				if b == a || cb.closed || len(cb.at) == 0 {
					continue
				}
				if d := gap(end, cb.at[0]); d < near {
					best, near = b, d
				}
			}
			if best < 0 {
				continue
			}
			cb := &chains[best]
			ca.at = append(ca.at[:len(ca.at)-1], cb.at...)
			ca.z = append(ca.z[:len(ca.z)-1], cb.z...)
			ca.face = append(ca.face[:len(ca.face)-1], cb.face...)
			cb.at, cb.z, cb.face = nil, nil, nil
			joined = true
		}
		if !joined {
			break
		}
	}
	// points nearer than a step of the edge are one
	for c := range chains {
		ch := &chains[c]
		var at []geom.Vec
		var z []float32
		var fs []int
		for i, q := range ch.at {
			if n := len(at); n > 0 && gap(at[n-1], q) < 0.02*m {
				continue
			}
			at, z, fs = append(at, q), append(z, ch.z[i]), append(fs, ch.face[i])
		}
		if n := len(at); ch.closed && n > 1 && gap(at[n-1], at[0]) < 0.02*m {
			at, z, fs = at[:n-1], z[:n-1], fs[:n-1]
		}
		ch.at, ch.z, ch.face = at, z, fs
	}
	return slices.DeleteFunc(chains, func(ch chain) bool {
		return len(ch.at) < 2 || ch.closed && len(ch.at) < 3
	})
}

// small says the hole at is no island: smaller than holeMin, or a slot thinner than a lane's
// line either side would leave room for and no bigger than holeMax
func small(at []geom.Vec, m float32) bool {
	a := -area(at)
	round := float32(0)
	for i, p := range at {
		round += gap(p, at[(i+1)%len(at)])
	}
	return a < holeMin*m*m || a < holeMax*m*m && 2*a < slot*m*round
}

// simplify drops the points of ch lying nearer than tol to the edge without them, and as high
// as it within ztol: the steps a piece's edge makes against the next's
// the most a simplified edge turns at a point, radians, where the edge runs round a curve
const bendMost = 20 * math.Pi / 180

func simplify(ch chain, tol, ztol float32) chain {
	n := len(ch.at)
	if n < 4 {
		return ch
	}
	keep := make([]bool, n)
	var dp func(i, j int)
	dp = func(i, j int) {
		a, b := ch.at[i%n], ch.at[j%n]
		za, zb := ch.z[i%n], ch.z[j%n]
		d := geom.NewVec(b.X-a.X, b.Y-a.Y)
		l := f32.Hypot(d.X, d.Y)
		far, at := float32(0), -1
		for k := i + 1; k < j; k++ {
			q := ch.at[k%n]
			var off, t float32
			if l < f32.Eps(1) {
				off = gap(a, q)
			} else {
				off = f32.Abs(d.X*(q.Y-a.Y)-d.Y*(q.X-a.X)) / l
				t = min(max(((q.X-a.X)*d.X+(q.Y-a.Y)*d.Y)/(l*l), 0), 1)
			}
			// as far off as it lies off the edge, or off the edge's height times tol/ztol
			off = max(off, f32.Abs(ch.z[k%n]-(za+(zb-za)*t))*tol/ztol)
			if off > far {
				far, at = off, k
			}
		}
		// a stretch round a curve turns no more than bendMost at a point kept: halved where its
		// halves turn more than half that
		bent := false
		if at >= 0 && l > 4*tol {
			q := ch.at[at%n]
			u := f32.Atan2(q.Y-a.Y, q.X-a.X)
			v := f32.Atan2(b.Y-q.Y, b.X-q.X)
			bent = f32.Abs(f32.Remainder(v-u, 2*math.Pi)) > bendMost/2
		}
		if at >= 0 && (far > tol || bent) {
			keep[at%n] = true
			dp(i, at)
			dp(at, j)
		}
	}
	if ch.closed {
		// from the point farthest from the first, round and back
		far := 0
		for k := range n {
			if gap(ch.at[0], ch.at[k]) > gap(ch.at[0], ch.at[far]) {
				far = k
			}
		}
		keep[0], keep[far] = true, true
		dp(0, far)
		dp(far, n)
	} else {
		keep[0], keep[n-1] = true, true
		dp(0, n-1)
	}
	var out chain
	out.closed = ch.closed
	for k := range n {
		if keep[k] {
			out.at, out.z, out.face = append(out.at, ch.at[k]), append(out.z, ch.z[k]),
				append(out.face, ch.face[k])
		}
	}
	return out
}

// close fills every notch the roadway's edge ch makes into the roadway narrower across its
// mouth than notchW and running no farther along the edge than notchL: the edge from then on
// straight across its mouth
func (r *Renderer) close(g *grid, ch chain) chain {
	m := r.cfg.Metre
	n := len(ch.at)
	if n < 4 {
		return ch
	}
	turn := func(i int) float32 {
		a, b, c := ch.at[(i+n-1)%n], ch.at[i], ch.at[(i+1)%n]
		d1 := geom.NewVec(b.X-a.X, b.Y-a.Y)
		d2 := geom.NewVec(c.X-b.X, c.Y-b.Y)
		return f32.Atan2(d1.X*d2.Y-d1.Y*d2.X, d1.X*d2.X+d1.Y*d2.Y)
	}
	out := float32(15 * math.Pi / 180)
	gone := make([]bool, n)
	for i := range n {
		if !ch.closed && (i == 0 || i == n-1) || gone[i] || turn(i) < out {
			continue
		}
		run := float32(0)
		for step := 1; step < n; step++ {
			j := i + step
			if !ch.closed && j >= n-1 {
				break
			}
			run += gap(ch.at[(j-1)%n], ch.at[j%n])
			if run > notchL*m {
				break
			}
			if gone[j%n] || turn(j%n) < out || gap(ch.at[i], ch.at[j%n]) > notchW*m {
				continue
			}
			poly := []geom.Vec{}
			for k := i; k <= j; k++ {
				poly = append(poly, ch.at[k%n])
			}
			hz := make([]float32, len(poly))
			for k := range poly {
				hz[k] = ch.z[(i+k)%n]
			}
			if area(poly) >= 0 || -area(poly) > notchA*m*m ||
				!sunk(poly, hz, r.ground, m) {
				continue
			}
			f := ch.face[i]
			r.cur = g.faces[f].owner
			rgb := r.rgb(g, f)
			for _, t := range earcut(poly) {
				at := [3]geom.Vec{poly[t[0]], poly[t[1]], poly[t[2]]}
				h := [3]float32{ch.z[(i+t[0])%n], ch.z[(i+t[1])%n], ch.z[(i+t[2])%n]}
				for q := range h {
					h[q] = max(h[q], r.ground(at[q])+lift)
				}
				mid := geom.NewVec((at[0].X+at[1].X+at[2].X)/3, (at[0].Y+at[1].Y+at[2].Y)/3)
				g.lay(at, h)
				r.tri(r.cfg.Sector(mid.X, mid.Y), at, h, rgb)
			}
			for k := i + 1; k < j; k++ {
				gone[k%n] = true
			}
			break
		}
	}
	var res chain
	res.closed = ch.closed
	for k := range n {
		if !gone[k] {
			res.at, res.z, res.face = append(res.at, ch.at[k]), append(res.z, ch.z[k]),
				append(res.face, ch.face[k])
		}
	}
	return res
}

// sunk says the ground within the outline at lies under the road round it, lying at z: no rise
// a road was laid past, which filled would lie buried
func sunk(at []geom.Vec, z []float32, ground func(geom.Vec) float32, m float32) bool {
	var mid geom.Vec
	low := float32(math.MaxFloat32)
	for i, q := range at {
		mid = geom.NewVec(mid.X+q.X/float32(len(at)), mid.Y+q.Y/float32(len(at)))
		low = min(low, z[i]-lift)
	}
	if ground(mid) > low+cutBy*m {
		return false
	}
	for _, q := range at {
		if ground(geom.NewVec((q.X+mid.X)/2, (q.Y+mid.Y)/2)) > low+cutBy*m {
			return false
		}
	}
	return true
}

// rgb is the surface of the face fi's piece
func (r *Renderer) rgb(g *grid, fi int) [3]float32 {
	f := &g.faces[fi]
	p := r.pieces[f.sector][f.index*floats : (f.index+1)*floats]
	return [3]float32{p[16], p[17], p[18]}
}

// fill lays the hole ch closes as roadway
func (r *Renderer) fill(g *grid, ch chain, ground func(geom.Vec) float32) {
	f := &g.faces[ch.face[0]]
	r.cur = f.owner
	rgb := r.rgb(g, ch.face[0])
	for _, t := range earcut(ch.at) {
		at := [3]geom.Vec{ch.at[t[0]], ch.at[t[1]], ch.at[t[2]]}
		h := [3]float32{ch.z[t[0]], ch.z[t[1]], ch.z[t[2]]}
		for q := range h {
			h[q] = max(h[q], ground(at[q])+lift)
		}
		mid := geom.NewVec((at[0].X+at[1].X+at[2].X)/3, (at[0].Y+at[1].Y+at[2].Y)/3)
		g.lay(at, h)
		r.tri(r.cfg.Sector(mid.X, mid.Y), at, h, rgb)
	}
}

// along is where on ch the way s from its start lies: the place, how high, and the stretch
func along(ch *chain, pos []float32, s float32) (geom.Vec, float32, int) {
	n := len(ch.at)
	segs := n - 1
	if ch.closed {
		segs = n
	}
	for k := range segs {
		if s > pos[k+1] && k < segs-1 {
			continue
		}
		a, b := ch.at[k], ch.at[(k+1)%n]
		run := pos[k+1] - pos[k]
		t := float32(0)
		if run > 0 {
			t = min(max((s-pos[k])/run, 0), 1)
		}
		return geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t),
			ch.z[k] + (ch.z[(k+1)%n]-ch.z[k])*t, k
	}
	return ch.at[0], ch.z[0], 0
}

// fillet rounds every corner the roadway's edge ch turns into the roadway at: the turns of
// the edge nearer each other than a corner is taken as one, the corner filled out to a curve
// tangent to the edge either side of it, as far as filletR warrants and the edges either side
// let, the curve the edge from then on
func (r *Renderer) fillet(g *grid, ch chain, ground func(geom.Vec) float32) chain {
	m := r.cfg.Metre
	n := len(ch.at)
	if n < 3 {
		return ch
	}
	turnAt := func(at []geom.Vec, i int) float32 {
		k := len(at)
		a, b, c := at[(i+k-1)%k], at[i], at[(i+1)%k]
		d1 := geom.NewVec(b.X-a.X, b.Y-a.Y)
		d2 := geom.NewVec(c.X-b.X, c.Y-b.Y)
		return f32.Atan2(d1.X*d2.Y-d1.Y*d2.X, d1.X*d2.X+d1.Y*d2.Y)
	}
	turns := func() []float32 {
		t := make([]float32, len(ch.at))
		for i := range ch.at {
			if !ch.closed && (i == 0 || i == len(ch.at)-1) {
				continue
			}
			t[i] = turnAt(ch.at, i)
		}
		return t
	}
	turn := turns()
	least := float32(filletMin * math.Pi / 180)
	slight := float32(3 * math.Pi / 180)
	// a closed edge starts where it runs straightest, far from any turn
	if ch.closed {
		start, best := 0, float32(-1)
		for i := range n {
			if f32.Abs(turn[i]) >= slight {
				continue
			}
			d := float32(math.MaxFloat32)
			for j := range n {
				if f32.Abs(turn[j]) >= least {
					d = min(d, gap(ch.at[i], ch.at[j]))
				}
			}
			if d > best {
				best, start = d, i
			}
		}
		ch.at = append(slices.Clone(ch.at[start:]), ch.at[:start]...)
		ch.z = append(slices.Clone(ch.z[start:]), ch.z[:start]...)
		ch.face = append(slices.Clone(ch.face[start:]), ch.face[:start]...)
		turn = turns()
	}
	segs := n - 1
	if ch.closed {
		segs = n
	}
	pos := make([]float32, segs+1)
	for k := range segs {
		pos[k+1] = pos[k] + gap(ch.at[k], ch.at[(k+1)%n])
	}
	// the corners: runs of turns into the roadway, each no farther than group from the last,
	// turning by least at least all told; and the turns out of it, which no rounding passes
	type corner struct {
		first, last int
		turn        float32
	}
	var corners []corner
	for i := 0; i < n; i++ {
		if turn[i] > -slight {
			continue
		}
		c := corner{first: i, last: i, turn: turn[i]}
		for j := i + 1; j < n && turn[j] <= -slight && pos[j]-pos[c.last] < group*m &&
			pos[j]-pos[c.first] < 2*group*m; j++ {
			c.last, c.turn = j, c.turn+turn[j]
		}
		i = c.last
		if c.turn < -least {
			corners = append(corners, c)
		}
	}
	if len(corners) == 0 {
		return ch
	}
	out := func(i int) bool { return turn[i] > least }
	type round struct {
		at     corner
		s0, s1 float32
	}
	var rounds []round
	for ci, c := range corners {
		alpha := math.Pi - min(f32.Abs(c.turn), math.Pi*0.97)
		t := min(filletR*m/f32.Tan(alpha/2), filletMax*m)
		back, ahead := pos[c.first], pos[segs]-pos[c.last]
		for j := c.first - 1; j >= 0; j-- {
			if out(j) {
				back = pos[c.first] - pos[j]
				break
			}
		}
		for j := c.last + 1; j < n; j++ {
			if out(j) {
				ahead = pos[j] - pos[c.last]
				break
			}
		}
		if ci > 0 {
			back = min(back, (pos[c.first]-pos[corners[ci-1].last])/2)
		}
		if ci < len(corners)-1 {
			ahead = min(ahead, (pos[corners[ci+1].first]-pos[c.last])/2)
		}
		t = min(t, 0.98*back, 0.98*ahead)
		// no farther into a corner than the ground there lies under the road: a rounding laid
		// over ground standing higher, as in a cutting, would lie buried
		for t >= 0.05*m {
			a, za, _ := along(&ch, pos, pos[c.first]-t)
			b, zb, _ := along(&ch, pos, pos[c.last]+t)
			v := ch.at[c.first]
			mid := geom.NewVec((a.X+2*v.X+b.X)/4, (a.Y+2*v.Y+b.Y)/4)
			low := min(za, zb, ch.z[c.first]) - lift
			if ground(mid) <= low+cutBy*m && ground(geom.NewVec((mid.X+v.X)/2,
				(mid.Y+v.Y)/2)) <= low+cutBy*m {
				break
			}
			t /= 2
		}
		if t < 0.05*m {
			continue
		}
		// nothing is rounded past the board's edge, where the roadway runs on unseen
		a, _, _ := along(&ch, pos, pos[c.first]-t)
		b, _, _ := along(&ch, pos, pos[c.last]+t)
		if r.off != nil && (r.off(a) || r.off(b) || r.off(ch.at[c.first])) {
			continue
		}
		rounds = append(rounds, round{at: c, s0: pos[c.first] - t, s1: pos[c.last] + t})
	}
	if len(rounds) == 0 {
		return ch
	}
	var res chain
	res.closed = ch.closed
	emit := func(q geom.Vec, z float32, f int) {
		res.at, res.z, res.face = append(res.at, q), append(res.z, z), append(res.face, f)
	}
	next := 0
	skip := float32(-1)
	for k := range n {
		for next < len(rounds) && rounds[next].s0 < pos[min(k, segs)] {
			rd := rounds[next]
			next++
			a, za, ka := along(&ch, pos, rd.s0)
			b, zb, kb := along(&ch, pos, rd.s1)
			// the curve's middle: where the edge into the corner and the edge out of it meet
			din := geom.NewVec(ch.at[(ka+1)%n].X-ch.at[ka].X, ch.at[(ka+1)%n].Y-ch.at[ka].Y)
			dout := geom.NewVec(ch.at[(kb+1)%n].X-ch.at[kb].X, ch.at[(kb+1)%n].Y-ch.at[kb].Y)
			v := ch.at[rd.at.first]
			zv := ch.z[rd.at.first]
			if den := din.X*dout.Y - din.Y*dout.X; f32.Abs(den) > 1e-6*(din.X*din.X+din.Y*din.Y+
				dout.X*dout.X+dout.Y*dout.Y) {
				u := ((b.X-a.X)*dout.Y - (b.Y-a.Y)*dout.X) / den
				x := geom.NewVec(a.X+din.X*u, a.Y+din.Y*u)
				// no farther from the corner than the curve's ends are: edges near opposite,
				// as round the end of a slot, meet far off
				if u > 0 && gap(x, v) < max(gap(a, v), gap(b, v)) {
					v = x
					zv = (za + zb) / 2
				}
			}
			f := ch.face[ka]
			// the corner filled: the edge from a round to b, and back along the curve
			poly := []geom.Vec{a}
			hz := []float32{za}
			for j := ka + 1; j < n && pos[j] < rd.s1; j++ {
				poly, hz = append(poly, ch.at[j]), append(hz, ch.z[j])
			}
			poly, hz = append(poly, b), append(hz, zb)
			const steps = 10
			var curve []geom.Vec
			var cz []float32
			for q := 1; q < steps; q++ {
				t := float32(q) / steps
				c := geom.NewVec((1-t)*(1-t)*a.X+2*t*(1-t)*v.X+t*t*b.X,
					(1-t)*(1-t)*a.Y+2*t*(1-t)*v.Y+t*t*b.Y)
				z := (1-t)*(1-t)*za + 2*t*(1-t)*zv + t*t*zb
				curve, cz = append(curve, c), append(cz, max(z, ground(c)+lift))
			}
			for q := len(curve) - 1; q >= 0; q-- {
				poly, hz = append(poly, curve[q]), append(hz, cz[q])
			}
			for q := range poly {
				hz[q] = max(hz[q], ground(poly[q])+lift)
			}
			r.cur = g.faces[f].owner
			rgb := r.rgb(g, f)
			for _, t := range earcut(poly) {
				at := [3]geom.Vec{poly[t[0]], poly[t[1]], poly[t[2]]}
				h := [3]float32{hz[t[0]], hz[t[1]], hz[t[2]]}
				g.lay(at, h)
				r.tri(r.cfg.Sector(v.X, v.Y), at, h, rgb)
			}
			emit(a, za, f)
			for q := range curve {
				emit(curve[q], cz[q], f)
			}
			emit(b, zb, f)
			skip = rd.s1
		}
		if pos[min(k, segs)] <= skip {
			continue
		}
		emit(ch.at[k], ch.z[k], ch.face[k])
	}
	return res
}

// line lays the line along the roadway's edge ch, lineInset in from it: each stretch's line
// meeting the next's where they cross, or cut square where the edge turns sharply out
func (r *Renderer) line(g *grid, ch chain) {
	m := r.cfg.Metre
	n := len(ch.at)
	h := lineInset * m
	var zs []float32
	left := func(a, b geom.Vec) (geom.Vec, bool) {
		d := geom.NewVec(b.X-a.X, b.Y-a.Y)
		l := f32.Hypot(d.X, d.Y)
		if l < f32.Eps(1) {
			return geom.Vec{}, false
		}
		return geom.NewVec(-d.Y/l, d.X/l), true
	}
	var at []geom.Vec
	var z []float32
	var fs []int
	put := func(q geom.Vec, k int) {
		top := r.over(g, q, ch.z[k], g.faces[ch.face[k]].owner)
		at, z, fs = append(at, q), append(z, top), append(fs, ch.face[k])
	}
	for k := range n {
		var n0, n1 geom.Vec
		ok0, ok1 := false, false
		if k > 0 || ch.closed {
			n0, ok0 = left(ch.at[(k+n-1)%n], ch.at[k])
		}
		if k < n-1 || ch.closed {
			n1, ok1 = left(ch.at[k], ch.at[(k+1)%n])
		}
		p := ch.at[k]
		switch {
		case ok0 && ok1:
			bis := geom.NewVec(n0.X+n1.X, n0.Y+n1.Y)
			l := f32.Hypot(bis.X, bis.Y)
			c := float32(0)
			if l > 1e-3 {
				bis = geom.NewVec(bis.X/l, bis.Y/l)
				c = bis.X*n1.X + bis.Y*n1.Y
			}
			if c < 0.34 {
				put(geom.NewVec(p.X+n0.X*h, p.Y+n0.Y*h), k)
				put(geom.NewVec(p.X+n1.X*h, p.Y+n1.Y*h), k)
				continue
			}
			put(geom.NewVec(p.X+bis.X*h/c, p.Y+bis.Y*h/c), k)
		case ok0:
			put(geom.NewVec(p.X+n0.X*h, p.Y+n0.Y*h), k)
		case ok1:
			put(geom.NewVec(p.X+n1.X*h, p.Y+n1.Y*h), k)
		}
	}
	segs := len(at) - 1
	if ch.closed {
		segs = len(at)
	}
	for k := range segs {
		a, b := at[k], at[(k+1)%len(at)]
		side, ok := left(a, b)
		if !ok {
			continue
		}
		f := &g.faces[fs[k]]
		r.cur = f.owner
		half := geom.NewVec(side.X*lineHalf*m, side.Y*lineHalf*m)
		// as high as the roadway every step short enough to keep on it as it climbs and falls,
		// a piece as long as the roadway runs straight under it
		steps := max(int(f32.Ceil(gap(a, b)/(lineStep*m))), 1)
		zs = append(zs[:0], z[k])
		for q := 1; q < steps; q++ {
			t := float32(q) / float32(steps)
			p := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
			zs = append(zs, r.over(g, p, z[k]+(z[(k+1)%len(at)]-z[k])*t, f.owner))
		}
		zs = append(zs, z[(k+1)%len(at)])
		for i := 0; i < steps; {
			j := i + 1
			for j < steps && straightOn(zs, i, j+1, 0.01*m) {
				j++
			}
			t0, t1 := float32(i)/float32(steps), float32(j)/float32(steps)
			p0 := geom.NewVec(a.X+(b.X-a.X)*t0, a.Y+(b.Y-a.Y)*t0)
			p1 := geom.NewVec(a.X+(b.X-a.X)*t1, a.Y+(b.Y-a.Y)*t1)
			r.add(r.cfg.Sector(p0.X, p0.Y), p0, p1, half, half, zs[i]+lineLift, zs[j]+lineLift,
				0, 0, 0, 0, lineRGB, 0)
			i = j
		}
	}
}

// straightOn says the heights zs from i to j lie within tol of the straight line between the
// two
func straightOn(zs []float32, i, j int, tol float32) bool {
	for q := i + 1; q < j; q++ {
		lin := zs[i] + (zs[j]-zs[i])*float32(q-i)/float32(j-i)
		if f32.Abs(zs[q]-lin) > tol {
			return false
		}
	}
	return true
}

// metres: how long a piece of line is at most, and how far under the roadway it is raised from
const (
	lineStep = 1.5
	onTopBy  = 1.5
)

// bank lays what holds the roadway along its edge ch where it hangs over lower ground: at each
// stretch of the edge a bank down to its foot, or a wall, running out to nothing where the
// edge comes down to the ground
func (r *Renderer) bank(g *grid, ch chain, ground func(geom.Vec) float32) {
	m := r.cfg.Metre
	n := len(ch.at)
	segs := n - 1
	if ch.closed {
		segs = n
	}
	// the way out at each point halfway between its two stretches' ways out: the banks either
	// side of a point land at one foot, no gap between them round a bend
	out := make([]geom.Vec, n)
	for v := range n {
		var sum geom.Vec
		for _, k := range [2]int{v - 1, v} {
			if !ch.closed && (k < 0 || k >= segs) {
				continue
			}
			k = (k + n) % n
			d := geom.NewVec(ch.at[(k+1)%n].X-ch.at[k].X, ch.at[(k+1)%n].Y-ch.at[k].Y)
			if l := f32.Hypot(d.X, d.Y); l > f32.Eps(1) {
				sum = geom.NewVec(sum.X+d.Y/l, sum.Y-d.X/l)
			}
		}
		if l := f32.Hypot(sum.X, sum.Y); l > f32.Eps(1) {
			out[v] = geom.NewVec(sum.X/l, sum.Y/l)
		}
	}
	// the road whose edge a stretch is, and its style, where it is banked
	style := func(k int) (int, painter.Ribbon, bool) {
		owner := g.faces[ch.face[k]].owner
		nd := &r.nodes[owner]
		st := nd.style
		banked := !nd.sunk && !(nd.deck && (!nd.head || nd.lanes == 0)) &&
			(st.Bank.A != 0 || st.Wall.A != 0)
		return owner, st, banked
	}
	// where each point's bank lands, worked out once for the stretches either side of it
	type landing struct {
		off      geom.Vec
		low      float32
		wall, ok bool
		long     float32 // the shorter of its stretches
	}
	feet := make([]landing, n)
	for v := range n {
		k := v
		if k >= segs {
			k = segs - 1
		}
		_, st, banked := style(k)
		if !banked {
			continue
		}
		side := out[v]
		if side == (geom.Vec{}) {
			continue
		}
		f := &feet[v]
		f.off, f.low, f.wall, f.ok = r.foot(st, ch.at[v], side, 1, ch.z[v]-lift, ground)
		f.long = float32(math.Inf(1))
		for _, j := range [2]int{v - 1, v} {
			if !ch.closed && (j < 0 || j >= segs) {
				continue
			}
			j = (j + n) % n
			f.long = min(f.long, gap(ch.at[j], ch.at[(j+1)%n]))
		}
	}
	// a bank beside a wall, or one far out beside an edge on the ground, would be a spike: the
	// point is a wall, for the stretches either side of it alike
	for v := range n {
		f := &feet[v]
		if !f.ok || f.wall || f32.Hypot(f.off.X, f.off.Y) <= f.long {
			continue
		}
		beside := false
		for _, j := range [2]int{v - 1, v + 1} {
			if !ch.closed && (j < 0 || j >= n) {
				continue
			}
			o := &feet[(j+n)%n]
			beside = beside || o.wall || !o.ok
		}
		if beside {
			f.off = geom.NewVec(out[v].X*batter*m, out[v].Y*batter*m)
			f.low, f.wall = ground(ch.at[v])-buried*m, true
		}
	}
	for k := range segs {
		a, b := ch.at[k], ch.at[(k+1)%n]
		if gap(a, b) < f32.Eps(1) {
			continue
		}
		owner, st, banked := style(k)
		if !banked {
			continue
		}
		f0, f1 := feet[k], feet[(k+1)%n]
		if !f0.ok && !f1.ok {
			continue
		}
		r.mine, r.cur = owner, owner
		za, zb := ch.z[k], ch.z[(k+1)%n]
		off0, low0, off1, low1 := f0.off, f0.low, f1.off, f1.low
		if !f0.ok {
			off0, low0 = geom.Vec{}, za
		}
		if !f1.ok {
			off1, low1 = geom.Vec{}, zb
		}
		face := st.Bank
		if f0.wall || f1.wall || face.A == 0 {
			face = st.Wall
		}
		if face.A == 0 {
			face = st.Bank
		}
		sector := r.cfg.Sector(a.X, a.Y)
		r.add(sector, a, b, off0, off1, za, zb, low0, low1, 0, bank, straight(face), 0)
		span := r.spans[sector]
		r.spans[sector] = [2]float32{min(span[0], low0, low1), span[1]}
	}
}

// rims lays across every mouth a road runs into a roundabout by the broken line it gives way
// at: on along the rim's line across each gap the roadway's edge leaves along the rim
func (r *Renderer) rims(g *grid, chains []chain) {
	m := r.cfg.Metre
	for ci := range r.nodes {
		c := &r.nodes[ci]
		if c.round == 0 {
			continue
		}
		on := func(q geom.Vec) bool {
			return f32.Abs(gap(q, c.at)-c.round) < 0.012*c.round+0.05*m
		}
		angle := func(q geom.Vec) float32 { return f32.Atan2(q.Y-c.at.Y, q.X-c.at.X) }
		// the stretches of the rim the edge runs along, as the angles they lie between
		type span struct{ from, to float32 }
		var spans []span
		for _, ch := range chains {
			n := len(ch.at)
			if n < 2 {
				continue
			}
			var ons []bool
			any := false
			for _, q := range ch.at {
				o := on(q)
				ons, any = append(ons, o), any || o
			}
			if !any {
				continue
			}
			// a run starts where the edge comes onto the rim
			for k := range n {
				prev := k - 1
				if prev < 0 {
					if !ch.closed {
						prev = -1
					} else {
						prev = n - 1
					}
				}
				if !ons[k] || prev >= 0 && ons[prev] {
					continue
				}
				lo, hi := angle(ch.at[k]), angle(ch.at[k])
				at, last := lo, k
				for j := k + 1; ; j++ {
					if !ch.closed && j >= n || j-k >= n {
						break
					}
					q := ch.at[j%n]
					if !ons[j%n] {
						break
					}
					a := angle(q)
					a = at + f32.Remainder(a-at, 2*math.Pi)
					at, last = a, j
					lo, hi = min(lo, a), max(hi, a)
				}
				if last > k {
					spans = append(spans, span{lo, hi})
				}
			}
		}
		if len(spans) == 0 {
			continue
		}
		for i := range spans {
			for spans[i].from < 0 {
				spans[i].from += 2 * math.Pi
				spans[i].to += 2 * math.Pi
			}
		}
		slices.SortFunc(spans, func(a, b span) int { return cmp.Compare(a.from, b.from) })
		// the gaps between them, round the rim
		reach := spans[0].to
		var gaps []span
		for _, sp := range spans[1:] {
			if sp.from > reach {
				gaps = append(gaps, span{reach, sp.from})
			}
			reach = max(reach, sp.to)
		}
		if first := spans[0].from + 2*math.Pi; first > reach {
			gaps = append(gaps, span{reach, first})
		}
		rad := c.round - lineInset*m
		r.cur = ci
		for _, gp := range gaps {
			long := (gp.to - gp.from) * rad
			if long < 0.5*m || gp.to-gp.from > math.Pi {
				continue
			}
			dashes := max(int(long/(2*rimDash*m)), 1)
			for q := range dashes {
				b0 := gp.from + (gp.to-gp.from)*(float32(q)+0.25)/float32(dashes)
				b1 := gp.from + (gp.to-gp.from)*(float32(q)+0.75)/float32(dashes)
				s0, c0 := f32.Sincos(b0)
				s1, c1 := f32.Sincos(b1)
				p0 := geom.NewVec(c.at.X+c0*rad, c.at.Y+s0*rad)
				p1 := geom.NewVec(c.at.X+c1*rad, c.at.Y+s1*rad)
				d := geom.NewVec(p1.X-p0.X, p1.Y-p0.Y)
				l := f32.Hypot(d.X, d.Y)
				if l < f32.Eps(1) {
					continue
				}
				side := geom.NewVec(-d.Y/l*2*lineHalf*m, d.X/l*2*lineHalf*m)
				z0 := r.over(g, p0, c.level(p0), ci)
				z1 := r.over(g, p1, c.level(p1), ci)
				r.add(r.cfg.Sector(p0.X, p0.Y), p0, p1, side, side, z0+lineLift, z1+lineLift,
					0, 0, 0, 0, lineRGB, 0)
			}
		}
	}
}

// onTop raises every line laid so it lies on the roadway as drawn under it: at either end and
// halfway, over the highest of the pieces there no farther under it than a road's step
func (r *Renderer) onTop() {
	m := r.cfg.Metre
	type tri struct {
		at     [3]geom.Vec
		z      [3]float32
		lo, hi geom.Vec
		disc   bool
		mid    geom.Vec
		radius float32
	}
	side := plot * m
	key := func(q geom.Vec) [2]int {
		return [2]int{int(f32.Floor(q.X / side)), int(f32.Floor(q.Y / side))}
	}
	grid := map[[2]int][]tri{}
	var lines [][]float32
	for _, sector := range slices.Sorted(maps.Keys(r.pieces)) {
		all := r.pieces[sector]
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			kind := int(p[13])
			if [3]float32(p[16:19]) == lineRGB {
				lines = append(lines, p)
				continue
			}
			if kind&(bank|island) != 0 || p[15] != 0 {
				continue
			}
			for _, f := range facesOf(p) {
				for t := range 2 {
					if !f.disc && t == 1 {
						break
					}
					tr := tri{at: f.tri[t], z: f.tz[t], lo: f.lo, hi: f.hi, disc: f.disc,
						mid: f.mid, radius: f.radius}
					a, b := key(tr.lo), key(tr.hi)
					for x := a[0]; x <= b[0]; x++ {
						for y := a[1]; y <= b[1]; y++ {
							grid[[2]int{x, y}] = append(grid[[2]int{x, y}], tr)
						}
					}
				}
			}
		}
	}
	// how far the roadway lies over z at q, of the pieces no farther under z than a step
	at := func(q geom.Vec, z float32) float32 {
		up := float32(0)
		for _, t := range grid[key(q)] {
			if q.X < t.lo.X || q.X > t.hi.X || q.Y < t.lo.Y || q.Y > t.hi.Y ||
				t.disc && gap(q, t.mid) > t.radius {
				continue
			}
			a, b, c := t.at[0], t.at[1], t.at[2]
			det := (b.Y-c.Y)*(a.X-c.X) + (c.X-b.X)*(a.Y-c.Y)
			if f32.Abs(det) < 1e-6 {
				continue
			}
			u := ((b.Y-c.Y)*(q.X-c.X) + (c.X-b.X)*(q.Y-c.Y)) / det
			v := ((c.Y-a.Y)*(q.X-c.X) + (a.X-c.X)*(q.Y-c.Y)) / det
			w := 1 - u - v
			if u < -1e-4 || v < -1e-4 || w < -1e-4 {
				continue
			}
			h := u*t.z[0] + v*t.z[1] + w*t.z[2]
			if d := h + lineLift - z; d > up && d < onTopBy*m {
				up = d
			}
		}
		return up
	}
	// and round it, as wide as the line: a line along where a piece ends over another lies on
	// the higher
	reach := 2 * lineHalf * m
	need := func(q geom.Vec, z float32) float32 {
		up := at(q, z)
		for _, d := range [4][2]float32{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			up = max(up, at(geom.NewVec(q.X+d[0]*reach, q.Y+d[1]*reach), z))
		}
		return up
	}
	for _, p := range lines {
		a, b := geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3])
		up0, up1 := need(a, p[8]), need(b, p[9])
		for _, t := range [3]float32{0.25, 0.5, 0.75} {
			q := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
			up := need(q, p[8]+(p[9]-p[8])*t)
			up0, up1 = max(up0, up), max(up1, up)
		}
		p[8] += up0
		p[9] += up1
	}
}

// unbank takes out every bank whose road's edge lies within the roadway: the edge of a road
// within a junction, or one a corner's rounding covers
func (r *Renderer) unbank(g *grid) {
	m := r.cfg.Metre
	for _, sector := range slices.Sorted(maps.Keys(r.pieces)) {
		all := r.pieces[sector]
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			if int(p[13])&bank == 0 {
				continue
			}
			out := geom.NewVec(p[4]+p[6], p[5]+p[7])
			l := f32.Hypot(out.X, out.Y)
			if l < f32.Eps(1) {
				continue
			}
			covered := 0
			for _, t := range [3]float32{0.2, 0.5, 0.8} {
				e := geom.NewVec(p[0]+(p[2]-p[0])*t, p[1]+(p[3]-p[1])*t)
				q := geom.NewVec(e.X+out.X/l*0.1*m, e.Y+out.Y/l*0.1*m)
				if r.paved(g, q, p[8]+(p[9]-p[8])*t, r.owners[sector][i/floats]) {
					covered++
				}
			}
			if covered >= 2 {
				p[4], p[5], p[6], p[7] = 0, 0, 0, 0
			}
		}
	}
}

// the places along a stretch its lanes' marks are tried at, how fast a road may widen or
// narrow, either side over its run, and keep its lanes marked, and the narrowest lane marked,
// metres
const (
	marked    = 8
	taper     = 0.12
	laneLeast = 2.75
)

// marks leaves the roadway's edge to the lines laid along it, no stretch marking its own, and
// a stretch's lanes unmarked where another road lies on it: the stretch cut where its marks
// run in under another road
func (r *Renderer) marks(g *grid) {
	m := r.cfg.Metre
	// whether the piece of owner o, part p, and the face f are one road: the same piece of
	// it, or the road running on through the next cell
	same := func(o, p int, f *face) bool {
		if f.owner == o {
			return f.part == p
		}
		if p > 0 {
			// a junction's arm, towards the node p-1
			if f.owner != p-1 {
				return false
			}
		} else if !slices.Contains(r.nodes[o].arms, f.owner) {
			return false
		}
		to := &r.nodes[f.owner]
		if f.part == 0 {
			return len(to.arms) < 3 && to.round == 0
		}
		return f.part == o+1
	}
	var cand []int
	for _, sector := range slices.Sorted(maps.Keys(r.pieces)) {
		count := len(r.pieces[sector]) / floats
		for index := range count {
			p := r.pieces[sector][index*floats : (index+1)*floats]
			kind := int(p[13])
			if p[12] <= 0 || kind&(disc|bank|marking) != 0 || p[15] != 0 ||
				p[4] == 0 && p[5] == 0 || p[6] == 0 && p[7] == 0 {
				continue
			}
			p[19] = float32(int(p[19]) | 3)
			// a stretch widening or narrowing fast is no lane's: the lanes end where the road
			// does
			w0, w1 := f32.Hypot(p[4], p[5]), f32.Hypot(p[6], p[7])
			if run := gap(geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3])); f32.Abs(w1-w0) >
				taper*run {
				p[19] = float32(int(p[19]) | 4)
				continue
			}
			// no more lanes marked than the road is wide for
			p[12] = min(p[12], max(1, f32.Floor(2*min(w0, w1)/(laneLeast*m))))
			o, part := r.owners[sector][index], r.parts[sector][index]
			a, b := geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3])
			lo := geom.NewVec(min(a.X, b.X)-m, min(a.Y, b.Y)-m)
			hi := geom.NewVec(max(a.X, b.X)+m, max(a.Y, b.Y)+m)
			cand = g.near(lo, hi, -1, cand)
			var under [marked]bool
			mixed := false
			for k := range marked {
				t := (float32(k) + 0.5) / marked
				q := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
				zq := p[8] + (p[9]-p[8])*t
				for _, fi := range cand {
					f := &g.faces[fi]
					if !same(o, part, f) && f.holds(q, 0.05*m) &&
						r.one(o, f.owner, f32.Abs(f.zAt(q)-zq)) {
						under[k] = true
						break
					}
				}
				mixed = mixed || under[k] != under[0]
			}
			if !mixed {
				if under[0] {
					p[19] = float32(int(p[19]) | 4)
				}
				continue
			}
			whole := slices.Clone(p)
			first := true
			for from := 0; from < marked; {
				to := from + 1
				for to < marked && under[to] == under[from] {
					to++
				}
				t0, t1 := float32(from)/marked, float32(to)/marked
				q := slices.Clone(whole)
				for _, pair := range [6][2]int{{0, 2}, {1, 3}, {4, 6}, {5, 7}, {8, 9}, {10, 11}} {
					x, y := whole[pair[0]], whole[pair[1]]
					q[pair[0]], q[pair[1]] = x+(y-x)*t0, x+(y-x)*t1
				}
				if under[from] {
					q[19] = float32(int(q[19]) | 4)
				}
				if first {
					copy(r.pieces[sector][index*floats:], q)
					first = false
				} else {
					r.pieces[sector] = append(r.pieces[sector], q...)
					r.owners[sector] = append(r.owners[sector], o)
					r.parts[sector] = append(r.parts[sector], part)
				}
				from = to
			}
		}
	}
}

// earcut is the triangles of the simple polygon at, as indices into it
func earcut(at []geom.Vec) [][3]int {
	n := len(at)
	if n < 3 {
		return nil
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	ccw := area(at) > 0
	cross := func(a, b, c geom.Vec) float32 {
		return (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
	}
	inside := func(q, a, b, c geom.Vec) bool {
		d1, d2, d3 := cross(a, b, q), cross(b, c, q), cross(c, a, q)
		if !ccw {
			d1, d2, d3 = -d1, -d2, -d3
		}
		return d1 > 0 && d2 > 0 && d3 > 0
	}
	var out [][3]int
	for len(idx) > 3 {
		cut := -1
		for k := range idx {
			i0, i1, i2 := idx[(k+len(idx)-1)%len(idx)], idx[k], idx[(k+1)%len(idx)]
			a, b, c := at[i0], at[i1], at[i2]
			cr := cross(a, b, c)
			if ccw && cr <= 0 || !ccw && cr >= 0 {
				continue
			}
			clear := true
			for _, j := range idx {
				if j != i0 && j != i1 && j != i2 && inside(at[j], a, b, c) {
					clear = false
					break
				}
			}
			if clear {
				cut = k
				break
			}
		}
		if cut < 0 {
			// what is left lies in a line: no ear but by force
			cut = 0
		}
		i0, i1, i2 := idx[(cut+len(idx)-1)%len(idx)], idx[cut], idx[(cut+1)%len(idx)]
		out = append(out, [3]int{i0, i1, i2})
		idx = append(idx[:cut], idx[cut+1:]...)
	}
	return append(out, [3]int{idx[0], idx[1], idx[2]})
}
