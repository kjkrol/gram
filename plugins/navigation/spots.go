package navigation

import (
	"cmp"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

// placeCells is how many cells round its target a group looks over for spots.
const placeCells = 64

// spacingGap is the room left between two boxes standing side by side, a share of the larger side:
// a box's width, so one of the group walks between two standing already.
const spacingGap = 1.0

// gapFor is the room left round a box of half size half.
func gapFor(half geom.Vec) float64 { return 2 * max(half.X, half.Y) * spacingGap }

// placed is where one unit of a group stands: spot, in cell; ok false where none was found.
type placed struct {
	spot geom.Vec
	cell board.CellID
	ok   bool
}

// spotSearch is the spots round a point, a cell at a time, the cheapest cell to reach first and,
// within one, the nearest the point first.
type spotSearch struct {
	k     *bodyKeeping
	at    geom.Vec
	step  geom.Vec // the lattice's spacing: the largest box and a gap
	tiles []board.CellID
	next  int
	cands []placed
	dist  []float64
}

// get is the i-th spot, found as far as it takes.
func (s *spotSearch) get(i int) (placed, bool) {
	for i >= len(s.cands) && s.next < len(s.tiles) {
		s.expand(s.tiles[s.next])
		s.next++
	}
	if i >= len(s.cands) {
		return placed{}, false
	}
	return s.cands[i], true
}

// expand adds the lattice points inside c, nearest the point first.
func (s *spotSearch) expand(c board.CellID) {
	k := s.k
	centre := k.grid.CellCenter(c)
	d := k.delta(s.at, centre)
	w, h := k.grid.CellBounds()
	x0, x1 := d.X-w/2, d.X+w/2
	y0, y1 := d.Y-h/2, d.Y+h/2
	from := len(s.cands)
	for i := math.Ceil(x0 / s.step.X); i*s.step.X <= x1; i++ {
		for j := math.Ceil(y0 / s.step.Y); j*s.step.Y <= y1; j++ {
			off := geom.NewVec(i*s.step.X, j*s.step.Y)
			p := k.fold(geom.NewVec(s.at.X+off.X, s.at.Y+off.Y))
			if in, ok := k.grid.CellAt(p); ok && in == c {
				s.cands = append(s.cands, placed{spot: p, cell: c, ok: true})
				s.dist = append(s.dist, math.Hypot(off.X, off.Y))
			}
		}
	}
	idx := make([]int, len(s.cands)-from)
	for n := range idx {
		idx[n] = from + n
	}
	slices.SortStableFunc(idx, func(a, b int) int { return cmp.Compare(s.dist[a], s.dist[b]) })
	sorted := make([]placed, len(idx))
	for n, i := range idx {
		sorted[n] = s.cands[i]
	}
	copy(s.cands[from:], sorted)
}

// place finds a spot for each of units round at on target, nearest at first: the target's cell,
// then the cheapest round it. Only the ground and the group count: nobody knows where the others
// stand, and whoever finds its spot taken learns it by striking the one there. Among units alike
// the spots are then dealt far side first, the unit furthest on along the way going deepest, so
// none has to pass one of its group standing already. The way the group comes is the second
// result.
func (k *bodyKeeping) place(units []member, at geom.Vec, target board.CellID) (out []placed, way geom.Vec) {
	return k.placeClearOf(units, at, target, nil)
}

// placeClearOf is place with no spot where struck stand.
func (k *bodyKeeping) placeClearOf(units []member, at geom.Vec, target board.CellID, struck []body) (out []placed, way geom.Vec) {
	out = make([]placed, len(units))
	if len(units) == 0 {
		return out, way
	}
	var half geom.Vec
	var dom board.Domain
	for _, u := range units {
		half = geom.NewVec(max(half.X, u.pos.Size.X/2), max(half.Y, u.pos.Size.Y/2))
		dom |= u.domain
	}
	gap := gapFor(half)
	order := make([]int, len(units))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		da, db := k.delta(at, units[a].centre()), k.delta(at, units[b].centre())
		return cmp.Compare(math.Hypot(da.X, da.Y), math.Hypot(db.X, db.Y))
	})
	s := &spotSearch{k: k, at: at, step: geom.NewVec(2*half.X+gap, 2*half.Y+gap), tiles: k.tilesRound(target, dom, units[order[0]].domain)}
	used := map[int]bool{}
	for _, ui := range order {
		for ci := 0; ; ci++ {
			c, ok := s.get(ci)
			if !ok {
				break
			}
			if used[ci] || !k.fits(units[ui], c.spot, struck) {
				continue
			}
			used[ci] = true
			out[ui] = c
			break
		}
	}
	way = k.wayOf(units, at)
	k.deepFirst(units, order, out, at, way)
	return out, way
}

// wayOf is the way units come towards at, from the middle of them; zero from on it.
func (k *bodyKeeping) wayOf(units []member, at geom.Vec) geom.Vec {
	var from geom.Vec
	for _, u := range units {
		d := k.delta(at, u.centre())
		from = geom.NewVec(from.X+d.X, from.Y+d.Y)
	}
	n := math.Hypot(from.X, from.Y)
	if n < 1e-9 {
		return geom.Vec{}
	}
	return geom.NewVec(-from.X/n, -from.Y/n)
}

// deepFirst deals the spots found among units alike again, a row at a time, deepest along way
// first: the units furthest on along way take the deepest row, and within a row they keep their
// order across the way. So a unit neither passes one standing already nor crosses another's way.
func (k *bodyKeeping) deepFirst(units []member, order []int, out []placed, at, way geom.Vec) {
	if way == (geom.Vec{}) {
		return
	}
	across := geom.NewVec(-way.Y, way.X)
	along := func(p, dir geom.Vec) float64 {
		d := k.delta(at, p)
		return d.X*dir.X + d.Y*dir.Y
	}
	type alike struct {
		size   geom.Vec
		domain board.Domain
		lift   float64
		height float64
	}
	classes := map[alike][]int{}
	var keys []alike
	for _, ui := range order {
		if !out[ui].ok {
			continue
		}
		u := units[ui]
		key := alike{u.pos.Size, u.domain, u.lift, u.z.Height}
		if _, seen := classes[key]; !seen {
			keys = append(keys, key)
		}
		classes[key] = append(classes[key], ui)
	}
	for _, key := range keys {
		members := slices.Clone(classes[key])
		spots := make([]placed, len(members))
		for i, ui := range members {
			spots[i] = out[ui]
		}
		row := 2 * max(key.size.X, key.size.Y) / 2 * (1 + spacingGap) / 2 // half a spacing: one row
		slices.SortStableFunc(spots, func(a, b placed) int { return cmp.Compare(along(b.spot, way), along(a.spot, way)) })
		slices.SortStableFunc(members, func(a, b int) int {
			return cmp.Compare(along(units[b].centre(), way), along(units[a].centre(), way))
		})
		for from := 0; from < len(spots); {
			to := from + 1
			for to < len(spots) && along(spots[from].spot, way)-along(spots[to].spot, way) < row {
				to++
			}
			rowSpots, rowUnits := spots[from:to], slices.Clone(members[from:to])
			slices.SortStableFunc(rowSpots, func(a, b placed) int { return cmp.Compare(along(a.spot, across), along(b.spot, across)) })
			slices.SortStableFunc(rowUnits, func(a, b int) int {
				return cmp.Compare(along(units[a].centre(), across), along(units[b].centre(), across))
			})
			for i, ui := range rowUnits {
				out[ui] = rowSpots[i]
			}
			from = to
		}
	}
}

// tilesRound is up to placeCells cells taking dom round target, the cheapest to reach first,
// priced for costs as a route is.
func (k *bodyKeeping) tilesRound(target board.CellID, dom, costs board.Domain) []board.CellID {
	type reach struct {
		c    board.CellID
		cost float64
	}
	best := map[board.CellID]float64{target: 0}
	done := map[board.CellID]bool{}
	frontier := []reach{{target, 0}}
	var out []board.CellID
	for len(frontier) > 0 && len(out) < placeCells {
		n := 0
		for i := range frontier {
			if frontier[i].cost < frontier[n].cost {
				n = i
			}
		}
		r := frontier[n]
		frontier = slices.Delete(frontier, n, n+1)
		if done[r.c] {
			continue
		}
		done[r.c] = true
		out = append(out, r.c)
		for _, nb := range k.grid.Neighbors(r.c) {
			kind := k.terrain.Kind(nb)
			if done[nb] || !kind.Admits(dom) {
				continue
			}
			cost := r.cost + kind.CostFor(costs)*k.grid.NeighborCost(r.c, nb)*k.climb(r.c, nb, costs)
			if b, seen := best[nb]; !seen || cost < b {
				best[nb] = cost
				frontier = append(frontier, reach{nb, cost})
			}
		}
	}
	return out
}

// climb is how many times as long the step from a to b takes in d for its slope.
func (k *bodyKeeping) climb(a, b board.CellID, d board.Domain) float64 {
	if k.finder.slopes == nil {
		return 1
	}
	return k.finder.slopes.Climb(a, b, d)
}

// fits reports whether u may stand at p: its box on cells its domain takes, all of them on the
// board, on no step, and clear of struck.
func (k *bodyKeeping) fits(u member, p geom.Vec, struck []body) bool {
	half := geom.NewVec(u.pos.Size.X/2, u.pos.Size.Y/2)
	in := geom.NewVec(half.X*0.999, half.Y*0.999)
	for _, s := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if _, ok := k.grid.CellAt(k.fold(geom.NewVec(p.X+s[0]*in.X, p.Y+s[1]*in.Y))); !ok {
			return false
		}
	}
	admitted := true
	k.grid.CellsUnder(boxAt(p, in), func(c board.CellID) {
		admitted = admitted && k.terrain.Kind(c).Admits(u.domain)
	})
	if !admitted || !k.level(u, p, in) {
		return false
	}
	gap := gapFor(half)
	for _, b := range struck {
		if b.domain&u.domain != 0 && !apart(k.delta(p, b.at), half, b.half, gap) {
			return false
		}
	}
	return true
}

// level reports whether u's box of half size in round p stands on no step: the top of the ground
// — its kind's Height standing on it — under every corner within half u's height of the top under
// p. A flyer, a unit of no height and a world of no heights stand anywhere.
func (k *bodyKeeping) level(u member, p, in geom.Vec) bool {
	if u.lift > 0 || u.z.Height <= 0 || k.ground == nil {
		return true
	}
	g := k.ground()
	if g == nil {
		return true
	}
	top := func(q geom.Vec) float64 {
		t := g.At(q)
		if c, ok := k.grid.CellAt(q); ok {
			t += k.terrain.Kind(c).Height
		}
		return t
	}
	mid := top(p)
	for _, s := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if math.Abs(top(k.fold(geom.NewVec(p.X+s[0]*in.X, p.Y+s[1]*in.Y)))-mid) > u.z.Height/2 {
			return false
		}
	}
	return true
}
