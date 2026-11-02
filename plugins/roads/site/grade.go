package site

import (
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
)

// Levelled is c's corners as the plan leaves them: heights, the ground's at c's CellOutline in its
// order before the plan is laid, with every corner the plan moves at its To.
func (e Estimate) Levelled(g grid.Grid, c cell.ID, heights []float32) []float32 {
	if len(e.Corners) == 0 {
		return heights
	}
	same := sameCorner * g.CellSpan()
	var buf [maxCorners]geom.Vec
	for k, p := range g.CellOutline(c, buf[:0]) {
		if k >= len(heights) {
			break
		}
		for _, m := range e.Corners {
			if f32.Abs(m.At.X-p.X) <= same && f32.Abs(m.At.Y-p.Y) <= same {
				heights[k] = m.To
				break
			}
		}
	}
	return heights
}

const (
	// the most corners a cell has on any grid, as ground's outlines hold
	maxCorners = 8
	// two cells' corners this near, of a cell's span, are one
	sameCorner = 1e-3
	// the relief's own, so the plan pulls the ground round the road as the shaping will
	shapeTolerance = 1e-3
	// where along a ramp a corner lies on the edge it enters or leaves by
	edgeT       = 1e-3
	gradeSweeps = 20000
	gradePushes = 1 << 20
)

// grade levels the ground under a Graded road as cmd/gramap's grade did on a square map, on either
// grid: the corners a cell's road holds level joined into classes, a class at the mean of its
// corners and within the Rules' cut and fill of each, the ramps between them no steeper than the
// Rules let them, a class with a corner that may not move held at it; then the road's cells are
// graded in turn as Shaper.Grade grades them, the ground round each following within step
func (d *draft) grade(step float32) error {
	s := newSurvey(d.g)
	var laid []int
	for i, c := range d.e.Cells {
		if d.laid[i] {
			s.add(c, 0)
			laid = append(laid, i)
		}
	}
	if len(laid) == 0 {
		return nil
	}
	s.reach(1)
	for j := range laid {
		sp := &s.spots[j]
		for k := range sp.n {
			s.road[sp.corner[k]] = true
		}
	}
	if err := d.level(s, laid); err != nil {
		return err
	}
	for v := range int32(len(s.at)) {
		if s.road[v] {
			if err := d.limits(s, v); err != nil {
				return err
			}
		}
	}
	if step > 0 {
		if err := d.follow(s, step, int32(len(laid))); err != nil {
			return err
		}
	}
	return d.settle(s, laid)
}

// level sets the road's corners: the classes, their relaxation within the Rules' cut and fill,
// the corners that may not move
func (d *draft) level(s *survey, laid []int) error {
	gr := d.gr
	n := int32(len(s.at))
	u := newClasses(n)
	type ramp struct {
		entry, exit int32
		limit, run  float64
		at          cell.ID
	}
	type side struct {
		corner, entry, exit int32
		t                   float64
		ramp                int
	}
	var ramps []ramp
	var sides []side
	tan := math.Tan(d.slope())
	for j, i := range laid {
		sp := &s.spots[j]
		links := d.e.Links[i]
		if links == 0 {
			continue
		}
		if way, ok := grid.Straight(gr, links); ok && !gr.Corner(way) {
			st := gr.Step(way)
			run2 := float64(st.X)*float64(st.X) + float64(st.Y)*float64(st.Y)
			var t [maxCorners]float64
			entry, exit := int32(-1), int32(-1)
			for k := range sp.n {
				p := sp.rel[k]
				// 0 on the edge the road enters by, 1 on the one it leaves by
				t[k] = (float64(p.X)*float64(st.X)+float64(p.Y)*float64(st.Y))/run2 + 0.5
				switch v := sp.corner[k]; {
				case t[k] < edgeT:
					if entry < 0 {
						entry = v
					}
					u.join(entry, v)
				case t[k] > 1-edgeT:
					if exit < 0 {
						exit = v
					}
					u.join(exit, v)
				}
			}
			if entry >= 0 && exit >= 0 {
				for k := range sp.n {
					if t[k] >= edgeT && t[k] <= 1-edgeT {
						sides = append(sides, side{sp.corner[k], entry, exit, t[k], len(ramps)})
					}
				}
				run := math.Sqrt(run2)
				ramps = append(ramps, ramp{entry, exit, 0.999 * run * tan, run, sp.c})
				continue
			}
		}
		for k := int32(1); k < sp.n; k++ {
			u.join(sp.corner[0], sp.corner[k])
		}
	}
	eps := float64(1e-4 * gr.CellSpan())
	cut, fill := float64(max(d.r.Cut, 0)), float64(max(d.r.Fill, 0))
	size := make([]float64, n)
	sum := make([]float64, n)
	val := make([]float64, n)
	// the least and the most a class may stand at, within the cut and fill of each of its corners
	lo := make([]float64, n)
	hi := make([]float64, n)
	frozen := make([]bool, n)
	for v := range n {
		if !s.road[v] {
			continue
		}
		root, h := u.find(v), float64(s.was[v])
		if size[root] == 0 {
			lo[root], hi[root] = h-cut, h+fill
		}
		size[root]++
		sum[root] += h
		lo[root], hi[root] = max(lo[root], h-cut), min(hi[root], h+fill)
		if !s.fixed[v] {
			continue
		}
		if frozen[root] && math.Abs(val[root]-h) > eps {
			return Refuse(gr, s.from[v], "lies on ground that may not move and does not fit it")
		}
		if !frozen[root] {
			frozen[root], val[root] = true, h
		}
	}
	inv := make([]float64, n)
	var free []int32
	for v := range n {
		if size[v] == 0 {
			continue
		}
		if lo[v] > hi[v]+eps || frozen[v] && (val[v] < lo[v]-eps || val[v] > hi[v]+eps) {
			return Refuse(gr, s.from[v], "lies on ground too uneven to level %.2f down and %.2f up",
				cut, fill)
		}
		if frozen[v] {
			continue
		}
		if lo[v] > hi[v] {
			lo[v], hi[v] = (lo[v]+hi[v])/2, (lo[v]+hi[v])/2
		}
		val[v] = min(max(sum[v]/size[v], lo[v]), hi[v])
		inv[v] = 1 / size[v]
		free = append(free, v)
	}
	var slabs []slab
	var planes []plane
	for _, rp := range ramps {
		if a, b := u.find(rp.entry), u.find(rp.exit); a != b {
			slabs = append(slabs, slab{a, b, rp.limit, rp.run, rp.at})
		}
	}
	for _, sd := range sides {
		c, a, b := u.find(sd.corner), u.find(sd.entry), u.find(sd.exit)
		rp := ramps[sd.ramp]
		switch {
		case a == b:
			if c != a {
				slabs = append(slabs, slab{c, a, 0, rp.run, rp.at})
			}
		case c == a || c == b:
			slabs = append(slabs, slab{a, b, 0, rp.run, rp.at})
		default:
			planes = append(planes, plane{c, a, b, sd.t})
		}
	}
	relaxClasses(val, inv, lo, hi, free, slabs, planes, 1e-6*float64(gr.CellSpan()))
	steep, over := -1, eps
	for j, sl := range slabs {
		if o := math.Abs(val[sl.b]-val[sl.a]) - sl.limit; o > over {
			steep, over = j, o
		}
	}
	if steep >= 0 {
		sl := slabs[steep]
		climb := math.Atan2(math.Abs(val[sl.b]-val[sl.a]), sl.run)
		return Refuse(gr, sl.at, "climbs %.1f°, %.1f° at most", inDegrees(climb),
			inDegrees(d.slope()))
	}
	snap := 1e-5 * gr.CellSpan()
	for v := range n {
		if !s.road[v] || s.fixed[v] {
			continue
		}
		if h := float32(val[u.find(v)]); f32.Abs(h-s.was[v]) > snap {
			s.now[v] = h
		}
	}
	var hs [maxCorners]float32
	for j, i := range laid {
		sp := &s.spots[j]
		for k := range sp.n {
			hs[k] = s.now[sp.corner[k]]
		}
		if !ground.RoadFits(gr, sp.c, hs[:sp.n], d.e.Links[i]) {
			return Refuse(gr, sp.c, "cannot be graded")
		}
		if err := d.climbs(sp.c, hs[:sp.n], d.e.Links[i]); err != nil {
			return err
		}
	}
	return nil
}

// limits refuses a corner dug or raised further than the Rules let it, but for a corner's height
// rounded to a float32
func (d *draft) limits(s *survey, v int32) error {
	dh, tol := s.now[v]-s.was[v], f32.Eps(s.was[v])
	switch {
	case dh == 0:
	case -dh > d.r.Cut+tol:
		return Refuse(d.gr, s.from[v], "digs a corner %.2f down, %.2f at most", -dh, d.r.Cut)
	case dh > d.r.Fill+tol:
		return Refuse(d.gr, s.from[v], "raises a corner %.2f, %.2f at most", dh, d.r.Fill)
	}
	return nil
}

// follow grades the road's laid cells in turn as Shaper.Grade grades a cell: its corners put where
// the plan has them and pinned, the ground round them following within step; refusing where that
// would leave a corner of the road off the plan
func (d *draft) follow(s *survey, step float32, laid int32) error {
	planned := slices.Clone(s.now)
	copy(s.now, s.was)
	spread := int32(max(d.r.Spread, 0))
	var queue []int32
	for j := range laid {
		sp := s.spots[j]
		queue = queue[:0]
		for k := range sp.n {
			if v := sp.corner[k]; s.now[v] != planned[v] {
				s.now[v] = planned[v]
				queue = append(queue, v)
			}
		}
		if err := d.ripple(s, &sp, queue, step, spread); err != nil {
			return err
		}
	}
	for v := range int32(len(planned)) {
		if s.road[v] && s.now[v] != planned[v] {
			return Refuse(d.gr, s.from[v], "would be pulled out of shape by the ground round it")
		}
	}
	return nil
}

// ripple pulls the ground round the corners in queue to within step of them, breadth first as the
// shaping's settle does, the cell sp's corners pinned; refusing where it would move what may not
// move or reach further from the road than spread cells
func (d *draft) ripple(s *survey, sp *spot, queue []int32, step float32, spread int32) error {
	gr := d.gr
	pinned := func(w int32) bool {
		for k := range sp.n {
			if sp.corner[k] == w {
				return true
			}
		}
		return false
	}
	for pushes := 0; len(queue) > 0; {
		v := queue[0]
		queue = queue[1:]
		s.reach(s.ring[v] + 1)
		h := s.now[v]
		for e := s.first[v]; e >= 0; e = s.next[e] {
			// a copy: reaching further below grows the spots
			o := s.spots[e/maxCorners]
			k := e % maxCorners
			for _, m := range [2]int32{(k + 1) % o.n, (k + o.n - 1) % o.n} {
				w := o.corner[m]
				if pinned(w) {
					continue
				}
				var want float32
				switch hw := s.now[w]; {
				case hw < h-step-shapeTolerance:
					want = h - step
				case hw > h+step+shapeTolerance:
					want = h + step
				default:
					continue
				}
				at := s.from[v]
				if !s.road[w] && s.ring[w] > spread {
					return Refuse(gr, at, "would reshape the ground %d cells off, %d at most",
						s.ring[w], spread)
				}
				s.reach(s.ring[w] + 1)
				if s.fixed[w] {
					return Refuse(gr, at, "would move ground that may not move")
				}
				if pushes++; pushes > gradePushes {
					return Refuse(gr, at, "cannot be graded")
				}
				s.now[w] = want
				if !s.road[w] {
					s.from[w] = at
				}
				queue = append(queue, w)
			}
		}
	}
	return nil
}

// settle notes what the levelling comes to, refusing where it would break a road standing or
// move more earth than the Rules let it
func (d *draft) settle(s *survey, laid []int) error {
	g, gr := d.g, d.gr
	// a corner's share of a cell's earth: a square's top as a bilinear patch, a hex's as six
	// triangles from its middle at the mean of its corners
	per := s.spots[0].area() / float64(s.spots[0].n)
	var cut, fill float64
	var lift, drop float32
	var corners []CornerMove
	for v := range int32(len(s.at)) {
		dh := s.now[v] - s.was[v]
		if dh == 0 {
			continue
		}
		if err := d.limits(s, v); err != nil {
			return err
		}
		earth := float64(f32.Abs(dh)) * float64(s.holders(v)) * per
		if dh > 0 {
			fill, lift = fill+earth, max(lift, dh)
		} else {
			cut, drop = cut+earth, max(drop, -dh)
		}
		corners = append(corners, CornerMove{At: s.at[v], From: s.was[v], To: s.now[v]})
	}
	var graded, blocked []cell.ID
	var before, after [maxCorners]float32
	for j := range s.spots {
		sp := &s.spots[j]
		moved := false
		for k := range sp.n {
			v := sp.corner[k]
			before[k], after[k] = s.was[v], s.now[v]
			moved = moved || after[k] != before[k]
		}
		if !moved {
			continue
		}
		if links := g.Road(sp.c); sp.ring > 0 && links != 0 &&
			ground.RoadFits(gr, sp.c, before[:sp.n], links) &&
			!ground.RoadFits(gr, sp.c, after[:sp.n], links) {
			return Refuse(gr, sp.c, "would be broken by the levelling")
		}
		graded = append(graded, sp.c)
		if g.Held(sp.c) {
			blocked = append(blocked, sp.c)
		}
	}
	if earth := cut + fill; earth > float64(d.r.Earth) {
		return Refuse(gr, d.e.Cells[laid[0]], "moves %.1f of earth, %.1f at most", earth,
			d.r.Earth)
	}
	d.e.Corners, d.e.Graded, d.e.Blocked = corners, graded, blocked
	d.e.Cut, d.e.Fill = float32(cut), float32(fill)
	d.e.Lift, d.e.Drop = lift, drop
	return nil
}

// relaxClasses moves the classes' values, in turn and in the weights of their sizes, until every
// slab and plane holds or none can be helped further, then puts each free class back within its
// lo and hi, sweep after sweep; inv is a class's inverse size, 0 for one held
func relaxClasses(val, inv, lo, hi []float64, free []int32, slabs []slab, planes []plane,
	tol float64) {
	last := math.Inf(1)
	for sweep := range gradeSweeps {
		worst := 0.0
		for j := range slabs {
			if sweep%2 == 1 {
				j = len(slabs) - 1 - j
			}
			p := slabs[j]
			wa, wb := inv[p.a], inv[p.b]
			d := val[p.b] - val[p.a]
			over := math.Abs(d) - p.limit
			if over <= 0 || wa+wb == 0 {
				continue
			}
			worst = max(worst, over)
			push := math.Copysign(over, d) / (wa + wb)
			val[p.a] += push * wa
			val[p.b] -= push * wb
		}
		for j := range planes {
			if sweep%2 == 1 {
				j = len(planes) - 1 - j
			}
			p := planes[j]
			ca, cb := 1-p.t, p.t
			on, at := ca*val[p.a]+cb*val[p.b], val[p.c]
			if inv[p.c] > 0 {
				at = min(max(on, lo[p.c]), hi[p.c])
				worst = max(worst, math.Abs(at-val[p.c]))
				val[p.c] = at
			}
			wa, wb := inv[p.a], inv[p.b]
			den := wa*ca*ca + wb*cb*cb
			if off := on - at; off != 0 && den > 0 {
				worst = max(worst, math.Abs(off))
				k := off / den
				val[p.a] -= k * wa * ca
				val[p.b] -= k * wb * cb
			}
		}
		for _, r := range free {
			v := min(max(val[r], lo[r]), hi[r])
			worst = max(worst, math.Abs(v-val[r]))
			val[r] = v
		}
		if worst <= tol {
			return
		}
		if sweep%1000 == 999 {
			if worst > 0.999*last {
				return // ramps no values within the classes' bounds reconcile
			}
			last = worst
		}
	}
}

// slab keeps two classes' values no more than limit apart: the edges of the ramp at, run long
type slab struct {
	a, b       int32
	limit, run float64
	at         cell.ID
}

// plane keeps class c's value at t of the way from a's to b's: a hex ramp's side corner, moved
// alone onto its ramp as far as its own bounds let it, the ramp's edges moving to meet the rest
type plane struct {
	c, a, b int32
	t       float64
}

// classes are the corners joined into classes, each its root's
type classes []int32

func newClasses(n int32) classes {
	u := make(classes, n)
	for i := range u {
		u[i] = int32(i)
	}
	return u
}

func (u classes) find(k int32) int32 {
	for u[k] != k {
		u[k] = u[u[k]]
		k = u[k]
	}
	return k
}

func (u classes) join(a, b int32) {
	if a, b = u.find(a), u.find(b); a != b {
		u[b] = a
	}
}

// survey is the ground round a road: the cells within some rings of it and their corners, each
// once however many cells share it
type survey struct {
	g     Ground
	gr    grid.Grid
	same  float32
	spots []spot
	slot  map[cell.ID]int32
	// where each ring of cells begins in spots, the road's own first
	rings []int32
	// by corner: where it lies, its height before and as planned, whether it may move, whether
	// the road's cells hold it, the nearest ring holding it, its first holder, the road's cell it
	// is moved for
	at    []geom.Vec
	was   []float32
	now   []float32
	fixed []bool
	road  []bool
	ring  []int32
	first []int32
	from  []cell.ID
	// by holder, spot*maxCorners+k: the next holder of the same corner
	next    []int32
	outline []geom.Vec
	heights []float32
}

type spot struct {
	c      cell.ID
	ring   int32
	n      int32
	corner [maxCorners]int32
	// the corners from the cell's middle, in CellOutline's order
	rel [maxCorners]geom.Vec
}

func newSurvey(g Ground) *survey {
	gr := g.Grid()
	return &survey{g: g, gr: gr, same: sameCorner * gr.CellSpan(), slot: map[cell.ID]int32{},
		rings: []int32{0}}
}

// add notes the cell c in ring, its corners one with those of the cells noted beside it; a corner
// is matched from the neighbour's middle a Step away, so a wrapping grid's seam matches too
func (s *survey) add(c cell.ID, ring int32) {
	gr := s.gr
	s.outline = gr.CellOutline(c, s.outline[:0])
	s.heights = s.g.Heights(c, s.heights[:0])
	mid := gr.CellCenter(c)
	sp := spot{c: c, ring: ring, n: int32(min(len(s.outline), maxCorners))}
	for k := range sp.n {
		p := s.outline[k]
		sp.rel[k] = geom.NewVec(p.X-mid.X, p.Y-mid.Y)
		sp.corner[k] = -1
	}
	for i := range gr.Directions() {
		x, ok := gr.Toward(c, i)
		if !ok {
			continue
		}
		j, ok := s.slot[x]
		if !ok {
			continue
		}
		o := &s.spots[j]
		st := gr.Step(i)
		for k := range sp.n {
			if sp.corner[k] >= 0 {
				continue
			}
			for m := range o.n {
				q := geom.NewVec(o.rel[m].X+st.X, o.rel[m].Y+st.Y)
				if f32.Abs(q.X-sp.rel[k].X) <= s.same && f32.Abs(q.Y-sp.rel[k].Y) <= s.same {
					sp.corner[k] = o.corner[m]
					break
				}
			}
		}
	}
	id := int32(len(s.spots))
	fixed := s.g.Fixed(c)
	for range maxCorners {
		s.next = append(s.next, -1)
	}
	for k := range sp.n {
		v := sp.corner[k]
		if v < 0 {
			v = int32(len(s.at))
			sp.corner[k] = v
			h := float32(0)
			if int(k) < len(s.heights) {
				h = s.heights[k]
			}
			s.at = append(s.at, s.outline[k])
			s.was = append(s.was, h)
			s.now = append(s.now, h)
			s.fixed = append(s.fixed, false)
			s.road = append(s.road, false)
			s.ring = append(s.ring, ring)
			s.first = append(s.first, -1)
			s.from = append(s.from, c)
		}
		s.ring[v] = min(s.ring[v], ring)
		s.fixed[v] = s.fixed[v] || fixed
		e := id*maxCorners + k
		s.next[e], s.first[v] = s.first[v], e
	}
	s.slot[c] = id
	s.spots = append(s.spots, sp)
}

// reach notes every cell within ring cells of the road
func (s *survey) reach(ring int32) {
	for k := int32(len(s.rings)); k <= ring; k++ {
		from, to := s.rings[k-1], int32(len(s.spots))
		s.rings = append(s.rings, to)
		for j := from; j < to; j++ {
			c := s.spots[j].c
			for i := range s.gr.Directions() {
				if n, ok := s.gr.Toward(c, i); ok {
					if _, seen := s.slot[n]; !seen {
						s.add(n, k)
					}
				}
			}
		}
	}
}

// holders is how many of the cells noted hold the corner v
func (s *survey) holders(v int32) int {
	n := 0
	for e := s.first[v]; e >= 0; e = s.next[e] {
		n++
	}
	return n
}

func (sp *spot) area() float64 {
	a := 0.0
	for k := range sp.n {
		p, q := sp.rel[k], sp.rel[(k+1)%sp.n]
		a += float64(p.X)*float64(q.Y) - float64(q.X)*float64(p.Y)
	}
	return math.Abs(a) / 2
}
