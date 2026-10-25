package spacing

import (
	"cmp"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/world"
)

const placeCells = 64

// SpacingGap is the room between two boxes side by side, a share of the larger side: a box's
// width, so one of the group walks between two standing already.
const SpacingGap = 1.0

func gapFor(half geom.Vec) float32 { return 2 * max(half.X, half.Y) * SpacingGap }

type placed struct {
	spot  geom.Vec
	cell  cell.ID
	level order.Level
	ok    bool
}

// the cheapest cell to reach first and, within one, the nearest the point first
type spotSearch struct {
	k     *BodyKeeping
	at    geom.Vec
	z     world.Z // aloft, or grounded
	level order.Level
	step  geom.Vec // the lattice's spacing: the largest box and a gap
	tiles []cell.ID
	next  int
	cands []placed
	dist  []float32
}

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

func (s *spotSearch) expand(c cell.ID) {
	k := s.k
	centre := k.grid.CellCenter(c)
	d := k.delta(s.at, centre)
	w, h := k.grid.CellBounds()
	x0, x1 := d.X-w/2, d.X+w/2
	y0, y1 := d.Y-h/2, d.Y+h/2
	from := len(s.cands)
	for i := f32.Ceil(x0 / s.step.X); i*s.step.X <= x1; i++ {
		for j := f32.Ceil(y0 / s.step.Y); j*s.step.Y <= y1; j++ {
			off := geom.NewVec(i*s.step.X, j*s.step.Y)
			p := k.fold(geom.NewVec(s.at.X+off.X, s.at.Y+off.Y))
			if in, ok := k.grid.CellAt(p); ok && in == c {
				on, level := s.on(p)
				s.cands = append(s.cands, placed{spot: p, cell: on, level: level, ok: true})
				s.dist = append(s.dist, f32.Hypot(off.X, off.Y))
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

func (s *spotSearch) on(p geom.Vec) (cell.ID, order.Level) {
	k := s.k
	f, ok := k.terrain.(interface {
		LevelFor(p geom.Vec, feet, reach float32) (cell.ID, bool, bool)
	})
	if !ok {
		c, _ := CellOf(k.grid, k.terrain, p, &s.z)
		return c, s.level
	}
	c, deck, _ := f.LevelFor(p, s.z.Altitude, s.z.Reach())
	switch {
	case s.level == order.EitherLevel:
		return c, order.EitherLevel
	case deck:
		return c, order.OnDeck
	}
	return c, order.OnGround
}

// only the ground and the group count: whoever finds its spot taken learns it by striking the one
// there
func (k *BodyKeeping) place(units []bodies.Member, at geom.Vec, target cell.ID,
	level order.Level) (out []placed, way geom.Vec) {
	return k.placeClearOf(units, at, target, level, nil)
}

func (k *BodyKeeping) placeClearOf(units []bodies.Member, at geom.Vec, target cell.ID,
	level order.Level, struck []bodies.Body) (out []placed, way geom.Vec) {
	out = make([]placed, len(units))
	if len(units) == 0 {
		return out, way
	}
	var half geom.Vec
	var dom cell.Domain
	for _, u := range units {
		half = geom.NewVec(max(half.X, u.Pos.Size.X/2), max(half.Y, u.Pos.Size.Y/2))
		dom |= u.Domain
	}
	gap := gapFor(half)
	byDistance := make([]int, len(units))
	for i := range byDistance {
		byDistance[i] = i
	}
	slices.SortStableFunc(byDistance, func(a, b int) int {
		da, db := k.delta(at, units[a].Centre()), k.delta(at, units[b].Centre())
		return cmp.Compare(f32.Hypot(da.X, da.Y), f32.Hypot(db.X, db.Y))
	})
	z := aloft
	if level == order.OnGround {
		z = grounded
	}
	s := &spotSearch{k: k, at: at, z: z, level: level,
		step:  geom.NewVec(2*half.X+gap, 2*half.Y+gap),
		tiles: k.finder.tilesRound(target, dom, units[byDistance[0]].Domain)}
	used := map[int]bool{}
	for _, ui := range byDistance {
		for ci := 0; ; ci++ {
			c, ok := s.get(ci)
			if !ok {
				break
			}
			if used[ci] || !k.fits(units[ui], c.spot, z, struck) {
				continue
			}
			used[ci] = true
			out[ui] = c
			break
		}
	}
	way = k.wayOf(units, at)
	k.deepFirst(units, byDistance, out, at, way)
	return out, way
}

// zero from on it
func (k *BodyKeeping) wayOf(units []bodies.Member, at geom.Vec) geom.Vec {
	var from geom.Vec
	for _, u := range units {
		d := k.delta(at, u.Centre())
		from = geom.NewVec(from.X+d.X, from.Y+d.Y)
	}
	n := f32.Hypot(from.X, from.Y)
	if n < f32.Eps(max(f32.Abs(at.X), f32.Abs(at.Y))) {
		return geom.Vec{}
	}
	return geom.NewVec(-from.X/n, -from.Y/n)
}

// the units furthest on along way take the deepest row, keeping their order across it, so none
// passes one standing already nor crosses another's way
func (k *BodyKeeping) deepFirst(units []bodies.Member, order []int, out []placed,
	at, way geom.Vec) {
	if way == (geom.Vec{}) {
		return
	}
	across := geom.NewVec(-way.Y, way.X)
	along := func(p, dir geom.Vec) float32 {
		d := k.delta(at, p)
		return d.X*dir.X + d.Y*dir.Y
	}
	type alike struct {
		size   geom.Vec
		domain cell.Domain
		lift   float32
		height float32
	}
	classes := map[alike][]int{}
	var keys []alike
	for _, ui := range order {
		if !out[ui].ok {
			continue
		}
		u := units[ui]
		key := alike{u.Pos.Size, u.Domain, u.Lift, u.Z.Height}
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
		row := 2 * max(key.size.X, key.size.Y) / 2 * (1 + SpacingGap) / 2 // half a spacing
		slices.SortStableFunc(spots, func(a, b placed) int { return cmp.Compare(along(b.spot, way), along(a.spot, way)) })
		slices.SortStableFunc(members, func(a, b int) int {
			return cmp.Compare(along(units[b].Centre(), way), along(units[a].Centre(), way))
		})
		for from := 0; from < len(spots); {
			to := from + 1
			for to < len(spots) && along(spots[from].spot, way)-along(spots[to].spot, way) < row {
				to++
			}
			rowSpots, rowUnits := spots[from:to], slices.Clone(members[from:to])
			slices.SortStableFunc(rowSpots, func(a, b placed) int { return cmp.Compare(along(a.spot, across), along(b.spot, across)) })
			slices.SortStableFunc(rowUnits, func(a, b int) int {
				return cmp.Compare(along(units[a].Centre(), across), along(units[b].Centre(), across))
			})
			for i, ui := range rowUnits {
				out[ui] = rowSpots[i]
			}
			from = to
		}
	}
}

func (k *BodyKeeping) fits(u bodies.Member, p geom.Vec, z world.Z, struck []bodies.Body) bool {
	half := geom.NewVec(u.Pos.Size.X/2, u.Pos.Size.Y/2)
	in := geom.NewVec(half.X*0.999, half.Y*0.999)
	for _, s := range [4][2]float32{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if _, ok := k.grid.CellAt(k.fold(geom.NewVec(p.X+s[0]*in.X, p.Y+s[1]*in.Y))); !ok {
			return false
		}
	}
	admitted := true
	if f, decks := k.terrain.(Footing); decks {
		for _, s := range [9][2]float32{
			{0, 0}, {-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1},
		} {
			q := k.fold(geom.NewVec(p.X+s[0]*in.X, p.Y+s[1]*in.Y))
			c, _ := k.grid.CellAt(q)
			admitted = admitted && f.KindFor(c, q, z.Altitude, z.Reach()).Admits(u.Domain)
		}
	} else {
		k.grid.CellsUnder(bodies.BoxAt(p, in), func(c cell.ID) {
			admitted = admitted && k.terrain.Kind(c).Admits(u.Domain)
		})
	}
	if !admitted || !k.level(u, p, in, z) {
		return false
	}
	gap := gapFor(half)
	for _, b := range struck {
		if b.Domain&u.Domain != 0 && !bodies.Apart(k.delta(p, b.At), half, b.Half, gap) {
			return false
		}
	}
	return true
}

// aloft's feet are over every deck, grounded's under every one
var (
	aloft    = world.Z{Altitude: f32.Inf(1)}
	grounded = world.Z{Altitude: f32.Inf(-1)}
)

// on no step: the top under every corner within half u's height of the top under p
func (k *BodyKeeping) level(u bodies.Member, p, in geom.Vec, z world.Z) bool {
	if u.Lift > 0 || u.Z.Height <= 0 || k.ground == nil {
		return true
	}
	g := k.ground()
	if g == nil {
		return true
	}
	surfaces, _ := g.(ground.Surfaces)
	top := func(q geom.Vec) float32 {
		t := g.At(q)
		if c, ok := k.grid.CellAt(q); ok {
			t += k.terrain.Kind(c).Height
		}
		if surfaces != nil {
			floor, _ := surfaces.Surface(q, u.Domain, z.Altitude, z.Reach())
			t = max(t, floor)
		}
		return t
	}
	mid := top(p)
	for _, s := range [4][2]float32{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if f32.Abs(top(k.fold(geom.NewVec(p.X+s[0]*in.X, p.Y+s[1]*in.Y)))-mid) > u.Z.Height/2 {
			return false
		}
	}
	return true
}
