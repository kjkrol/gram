package navigation

import (
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// stallAfter is how long a unit may come no nearer the cell it heads for before it counts as
// stalled.
const stallAfter = time.Second

// maxStalls is how often a unit may stall without reaching the cell it heads for before it gives
// its order up and stands where it is.
const maxStalls = 5

// asideTime is how long a unit steps round whoever it struck.
const asideTime = 400 * time.Millisecond

// asideGoal is how much of the way to its goal a unit keeps while it steps round someone.
const asideGoal = 0.5

// openOccupancy refuses nobody and holds nothing: under BodySpacing a step is only noted, and the
// boxes meet as they meet.
type openOccupancy struct{}

func (openOccupancy) CanEnter(board.CellID, uid.UID64, board.Domain) bool { return true }
func (openOccupancy) Enter(board.CellID, uid.UID64, board.Domain)         {}
func (openOccupancy) Leave(board.CellID, uid.UID64)                       {}
func (openOccupancy) Release(func(uid.UID64) bool)                        {}

// bodyKeeping keeps units apart by their boxes. A unit routes over the ground alone, not knowing
// where the others stand, and goes; striking someone is a Touch, and what it does then is the
// rules' (Crowd): its Detour steps round them towards its goal and notes the cell of one
// standing for its routes to go round. A group is given its spots round the point it is sent to.
type bodyKeeping struct {
	finder  *pathFinder
	grid    board.Grid
	terrain board.Terrain
	space   *aabbworld.Space     // for the short way round and the world's edges; nil, flat and open
	ground  func() board.Heights // the board's ground, for steps; nil or giving nil, level
	index   bodyIndex
}

var _ keeping = (*bodyKeeping)(nil)

// newBodyKeeping keeps units apart over finder's grid, in space, on ground.
func newBodyKeeping(finder *pathFinder, space *aabbworld.Space, ground func() board.Heights) *bodyKeeping {
	return &bodyKeeping{finder: finder, grid: finder.grid, terrain: finder.terrain, space: space, ground: ground, index: bodyIndex{grid: finder.grid}}
}

func (k *bodyKeeping) occupancy() board.Occupancy { return k.finder.occupancy }

func (k *bodyKeeping) begin(gather func([]body) []body) {
	k.index.build(gather(k.index.bodies[:0]))
}

// delta is the way from a to b, the short way round on a wrapping axis.
func (k *bodyKeeping) delta(a, b geom.Vec) geom.Vec {
	if k.space == nil {
		return geom.NewVec(b.X-a.X, b.Y-a.Y)
	}
	w, h, e := k.space.Bounds()
	return geom.NewVec(shortestAxisDelta(a.X, b.X, w, e.WrapsX()), shortestAxisDelta(a.Y, b.Y, h, e.WrapsY()))
}

// fold is p inside the world on a wrapping axis.
func (k *bodyKeeping) fold(p geom.Vec) geom.Vec {
	if k.space == nil {
		return p
	}
	w, h, e := k.space.Bounds()
	if e.WrapsX() {
		p = geom.NewVec(wrapAxis(p.X, float64(w)), p.Y)
	}
	if e.WrapsY() {
		p = geom.NewVec(p.X, wrapAxis(p.Y, float64(h)))
	}
	return p
}

func wrapAxis(v, size float64) float64 {
	v = math.Mod(v, size)
	if v < 0 {
		v += size
	}
	return v
}

// half is half m's box.
func half(m member) geom.Vec { return geom.NewVec(m.pos.Size.X/2, m.pos.Size.Y/2) }

// orders stands the members round cmd.At on cmd.Cell, or round its centre without a point, each
// with a route to its spot; with Append one under an order queues the spot instead.
func (k *bodyKeeping) orders(members []member, cmd MoveTo, issue func(member, MoveOrder)) {
	at := cmd.At
	if at == (geom.Vec{}) {
		at = k.grid.CellCenter(cmd.Cell)
	}
	var group []member
	for _, m := range members {
		if cmd.Append && m.order != nil || m.from == cmd.Cell || k.reaches(m, cmd.Cell) {
			group = append(group, m)
		}
	}
	spots, _ := k.place(group, at, cmd.Cell)
	for i, p := range spots {
		m := group[i]
		if !p.ok {
			continue
		}
		if cmd.Append && m.order != nil {
			m.order.Enqueue(Goal{Cell: p.cell, Spot: p.spot, At: at})
			continue
		}
		path, ok := Path{}, m.from == p.cell
		if !ok {
			path, ok = k.finder.findPath(m.id, m.domain, m.from, p.cell)
		}
		if ok {
			issue(m, MoveOrder{Target: p.cell, Spot: p.spot, At: at, Path: path, Leg: m.leg})
		}
	}
}

// reaches reports whether any route over the ground takes m to c.
func (k *bodyKeeping) reaches(m member, c board.CellID) bool {
	_, ok := k.finder.findPath(m.id, m.domain, m.from, c)
	return ok
}

// look stops m where its braking ends, or where it stands when the ground there does not take it,
// and turns it.
func (k *bodyKeeping) look(m member, at geom.Vec) MoveOrder {
	here := m.centre()
	stop := here
	if v := math.Hypot(m.vel.X, m.vel.Y); v > 0 && m.brake > 0 {
		d := v / (2 * m.brake)
		stop = k.fold(geom.NewVec(here.X+m.vel.X*d, here.Y+m.vel.Y*d))
	}
	if !k.fits(m, stop, nil) {
		stop = here
	}
	cell, ok := k.grid.CellAt(stop)
	if !ok {
		cell = m.cell
	}
	return MoveOrder{Target: cell, Spot: stop, At: stop, Face: at}
}

// ready places an order given without a spot — at spawn, or a queued goal given none — round its
// point, or its Target's centre.
func (k *bodyKeeping) ready(m member, o *MoveOrder) {
	if o.Spot != (geom.Vec{}) {
		return
	}
	k.placeAgain(m, o, nil)
}

// placeAgain stands m elsewhere round o's point, or its Target's centre, clear of struck.
func (k *bodyKeeping) placeAgain(m member, o *MoveOrder, struck []body) {
	at := o.At
	if at == (geom.Vec{}) {
		at = k.grid.CellCenter(o.Target)
	}
	spots, _ := k.placeClearOf([]member{m}, at, o.Target, struck)
	p := spots[0]
	if !p.ok {
		p = placed{spot: k.grid.CellCenter(o.Target), cell: o.Target}
	}
	if p.cell != o.Target {
		o.Path, o.Leg = Path{}, Leg{}
	}
	o.Target, o.Spot, o.At = p.cell, p.spot, at
}

// route is the way over the ground to o's Target, round the cells m found someone standing in.
func (k *bodyKeeping) route(m member, from board.CellID, o *MoveOrder) (Path, bool) {
	if o.Avoids > 0 {
		known := o.Avoid[:o.Avoids]
		blocked := func(c board.CellID) bool { return c != o.Target && slices.Contains(known, c) }
		if p, ok := k.finder.findPathAround(m.id, m.domain, from, o.Target, blocked); ok {
			return p, true
		}
	}
	return k.finder.findPath(m.id, m.domain, from, o.Target)
}

// lost waits targetWaitTimeout for the ground to change, then stands round the point on a cell a
// route reaches, or gives up.
func (k *bodyKeeping) lost(m member, from board.CellID, o *MoveOrder, waited time.Duration) (board.CellID, Path, bool, bool) {
	if waited < targetWaitTimeout {
		return 0, Path{}, true, false
	}
	for _, c := range k.tilesRound(o.Target, m.domain, m.domain) {
		if c == from {
			return c, Path{}, false, true
		}
		if p, ok := k.finder.findPath(m.id, m.domain, from, c); ok {
			o.Spot = geom.Vec{} // placed round the point in the new cell next tick
			return c, p, false, true
		}
	}
	return 0, Path{}, false, false
}

// steer asks st for dir at speed; while m steps round someone it struck, dir leans aside, never
// back into them, and while m still faces into them it turns on the spot; with no ground aside
// either way it waits.
func (k *bodyKeeping) steer(m member, st steering.Helm, dir geom.Vec, speed float64) {
	if o := m.order; o != nil && o.AsideFor > 0 {
		if o.Aside == (geom.Vec{}) || m.facing.X*o.Struck.X+m.facing.Y*o.Struck.Y < -0.01 {
			speed = 0
		}
		d := o.Aside
		if n := math.Hypot(dir.X, dir.Y); n > 0 {
			d = geom.NewVec(d.X+dir.X/n*asideGoal, d.Y+dir.Y/n*asideGoal)
		}
		if into := -(d.X*o.Struck.X + d.Y*o.Struck.Y); into > 0 {
			d = geom.NewVec(d.X+o.Struck.X*into, d.Y+o.Struck.Y*into)
		}
		if n := math.Hypot(d.X, d.Y); n > 1e-9 && k.takes(m, geom.NewVec(d.X/n, d.Y/n)) {
			dir = geom.NewVec(d.X/n, d.Y/n)
		}
	}
	if dir != (geom.Vec{}) {
		st.Request(dir)
	}
	st.RequestSpeed(speed)
}

// watch ends a step round someone once its time is up, and marks o Bumped once m has come no
// nearer toward's point want by a quarter of its size for stallAfter; heading for another cell
// starts afresh, and reaching the one it headed for forgets the stalls.
func (k *bodyKeeping) watch(m member, o *MoveOrder, toward board.CellID, want geom.Vec, d time.Duration) {
	if o.AsideFor = max(o.AsideFor-d, 0); o.AsideFor == 0 {
		o.Aside, o.Struck = geom.Vec{}, geom.Vec{}
	}
	c := m.centre()
	dist := math.Hypot(want.X-c.X, want.Y-c.Y)
	if m.cell == o.Toward && toward != o.Toward {
		o.Stalls = 0
	}
	if toward != o.Toward || o.Closest == 0 && o.Stalled == 0 {
		o.Toward, o.Closest, o.Stalled = toward, dist, 0
		return
	}
	if dist < o.Closest-min(m.pos.Size.X, m.pos.Size.Y)/4 {
		o.Closest, o.Stalled = dist, 0
		return
	}
	if o.Stalled += d; o.Stalled >= stallAfter {
		o.Bumped, o.Stalled, o.Struck, o.Hit, o.HitUnit, o.AsideFor = true, 0, geom.Vec{}, 0, false, 0
	}
}

// bump answers a stall with a fresh route, and gives up after maxStalls; and it answers the solid
// ground struck — Struck the way off it — by stepping round it towards m's goal.
func (k *bodyKeeping) bump(m member, o *MoveOrder) bumpAnswer {
	if o.Struck == (geom.Vec{}) {
		if o.Stalls++; o.Stalls > maxStalls {
			return giveUp
		}
		o.Path, o.Leg, o.Closest = Path{}, Leg{}, math.Inf(1)
		return carryOn
	}
	k.stepRound(m, o, o.Struck)
	return carryOn
}

// detour has m step round other towards its goal. One standing is noted met — its spot kept clear
// of when m stands elsewhere — and its cell for the routes to go round, the route planned afresh
// when it runs through it; going round again one it knew stands there, the last step round over,
// is no headway: a stall, giving up past maxStalls.
func (k *bodyKeeping) detour(m member, o *MoveOrder, other body) bumpAnswer {
	if !other.moving && other.domain&m.domain != 0 {
		if slices.Contains(o.Met[:o.Mets], other.id) && o.AsideFor == 0 {
			if o.Stalls++; o.Stalls > maxStalls {
				return giveUp
			}
		}
		o.meet(other.id)
		if c, ok := k.grid.CellAt(other.at); ok && c != o.Target && c != m.cell {
			o.learn(c)
			if o.Path.Index < o.Path.Length && slices.Contains(o.Path.Steps[o.Path.Index:o.Path.Length], c) || o.Leg.Active && o.Leg.To == c {
				o.Path, o.Leg = Path{}, Leg{}
			}
		}
	}
	if n := k.wayOff(m, other); n != (geom.Vec{}) {
		k.stepRound(m, o, n)
	}
	return carryOn
}

// pass has m step round other a while, its route kept: other makes way for it.
func (k *bodyKeeping) pass(m member, o *MoveOrder, other body) {
	if n := k.wayOff(m, other); n != (geom.Vec{}) {
		k.stepRound(m, o, n)
	}
}

// wayOff is the way m leaves other as the collision parts two boxes: square off the side of other
// it is least into; zero on top of each other.
func (k *bodyKeeping) wayOff(m member, other body) geom.Vec {
	d := k.delta(other.at, m.centre())
	h := half(m)
	intoX := h.X + other.half.X - math.Abs(d.X)
	intoY := h.Y + other.half.Y - math.Abs(d.Y)
	switch {
	case d == (geom.Vec{}):
		return geom.Vec{}
	case intoX < intoY:
		return geom.NewVec(math.Copysign(1, d.X), 0)
	default:
		return geom.NewVec(0, math.Copysign(1, d.Y))
	}
}

// stepRound has m step round whatever it struck, n the way off it, for asideTime: to the side its
// goal lies — head on, the left of the way off, so two meeting head on pass — where the ground a
// step along takes it, no steeper than yieldClimb, never back into it; with no ground aside either
// way it is only kept from backing in.
func (k *bodyKeeping) stepRound(m member, o *MoveOrder, n geom.Vec) {
	goal := k.delta(m.centre(), k.goalOf(o))
	aside := geom.NewVec(-n.Y, n.X)
	if l := math.Hypot(goal.X, goal.Y); l > 1e-9 && (aside.X*goal.X+aside.Y*goal.Y)/l < -0.2 {
		aside = geom.NewVec(-aside.X, -aside.Y)
	}
	if !k.takes(m, aside) {
		aside = geom.NewVec(-aside.X, -aside.Y)
		if !k.takes(m, aside) {
			aside = geom.Vec{}
		}
	}
	o.Struck, o.Aside, o.AsideFor = n, aside, asideTime
}

// takes reports whether the ground a step along dir from m, its box clear past its own edge,
// takes m, no steeper than yieldClimb from where it stands.
func (k *bodyKeeping) takes(m member, dir geom.Vec) bool {
	h := half(m)
	reach := 2*max(h.X, h.Y) + gapFor(h)
	c := m.centre()
	at := k.fold(geom.NewVec(c.X+dir.X*reach, c.Y+dir.Y*reach))
	ok := true
	k.grid.CellsUnder(boxAt(at, h), func(cell board.CellID) {
		ok = ok && k.terrain.Kind(cell).Admits(m.domain) && k.finder.climb(m.cell, cell, m.domain) <= yieldClimb
	})
	for _, s := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if _, in := k.grid.CellAt(k.fold(geom.NewVec(at.X+s[0]*h.X, at.Y+s[1]*h.Y))); !in {
			ok = false
		}
	}
	return ok
}

// met is the ones o's unit struck standing, where they are now.
func (k *bodyKeeping) met(o *MoveOrder) []body {
	var out []body
	for _, id := range o.Met[:o.Mets] {
		if b, ok := k.index.of(id); ok {
			out = append(out, b)
		}
	}
	return out
}

// goalOf is where o ends: its Spot, or its Target's centre.
func (k *bodyKeeping) goalOf(o *MoveOrder) geom.Vec {
	if o.Spot != (geom.Vec{}) {
		return o.Spot
	}
	return k.grid.CellCenter(o.Target)
}

// yieldLinger is how long a unit stepping aside on the move stands there before it goes on.
const yieldLinger = time.Second

// blocked is never called: the open occupancy refuses nobody.
func (k *bodyKeeping) blocked(member, *MoveOrder, board.CellID, time.Duration) (uid.UID64, bool) {
	return 0, false
}

func (k *bodyKeeping) byContact() bool { return true }

// within is a quarter of m's shorter side, at most arrivalEpsilon: arriving it is put on its spot.
func (k *bodyKeeping) within(m member) float64 {
	return min(arrivalEpsilon, min(m.pos.Size.X, m.pos.Size.Y)/4)
}

// mayStep lets m on unless it all but touches someone just ahead along heading: walked by hand, it
// stops at whoever it walks into.
func (k *bodyKeeping) mayStep(m member, _ board.CellID, heading geom.Vec) bool {
	c, h := m.centre(), half(m)
	feel := gapFor(h) / 4
	ahead := geom.NewVec(c.X+heading.X*feel, c.Y+heading.Y*feel)
	open := true
	k.index.near(boxAt(ahead, geom.NewVec(h.X+k.index.largest, h.Y+k.index.largest)), func(b *body) {
		if !open || b.id == m.id || b.domain&m.domain == 0 {
			return
		}
		d := k.delta(c, b.at)
		if d.X*heading.X+d.Y*heading.Y > 0 && !apart(k.delta(ahead, b.at), h, b.half, 0) {
			open = false
		}
	})
	return open
}

// other is whoever id was this tick.
func (k *bodyKeeping) other(id uid.UID64) (body, bool) { return k.index.of(id) }

// asides are the two places m could step to off asker's way, square to the line between them —
// to the side m stands first — as far as the two boxes and a gap need.
func (k *bodyKeeping) asides(m member, asker body) [2]geom.Vec {
	here, h := m.centre(), half(m)
	way := k.delta(asker.at, here)
	if l := math.Hypot(way.X, way.Y); l > 1e-9 {
		way = geom.NewVec(way.X/l, way.Y/l)
	} else {
		way = geom.NewVec(1, 0)
	}
	across := geom.NewVec(-way.Y, way.X)
	off := k.delta(asker.at, here)
	side := off.X*across.X + off.Y*across.Y
	sign := 1.0
	if side < 0 {
		sign = -1
	}
	reach := func(half geom.Vec) float64 { return half.X*math.Abs(across.X) + half.Y*math.Abs(across.Y) }
	clear := reach(asker.half) + reach(h) + gapFor(h)/4
	var out [2]geom.Vec
	for i, step := range [2]float64{sign * (clear - math.Abs(side)), -sign * (clear + math.Abs(side))} {
		if math.Abs(step) < gapFor(h)/4 {
			step = math.Copysign(gapFor(h)/4, step)
		}
		out[i] = k.fold(geom.NewVec(here.X+across.X*step, here.Y+across.Y*step))
	}
	return out
}

// open reports whether m may stand at p — the ground takes it, no steeper than yieldClimb from
// where it stands — and who stands in the way there first, taken when someone does.
func (k *bodyKeeping) open(m member, p geom.Vec) (ground bool, in uid.UID64, taken bool) {
	cell, ok := k.grid.CellAt(p)
	if !ok || !k.fits(m, p, nil) || k.finder.climb(m.cell, cell, m.domain) > yieldClimb {
		return false, 0, false
	}
	h := half(m)
	k.index.near(boxAt(p, geom.NewVec(h.X+k.index.largest, h.Y+k.index.largest)), func(b *body) {
		if !taken && b.id != m.id && b.domain&m.domain != 0 && !apart(k.delta(p, b.at), h, b.half, 0) {
			in, taken = b.id, true
		}
	})
	return true, in, taken
}

// stepAside has m step just off of's way, square to the line between them — to the side m stands
// first — where open says it may stand, facing as it did; with neither side open it has no order.
func (k *bodyKeeping) stepAside(m member, of body) (MoveOrder, bool) {
	for _, p := range k.asides(m, of) {
		if ground, _, taken := k.open(m, p); !ground || taken {
			continue
		}
		cell, _ := k.grid.CellAt(p)
		o := MoveOrder{Target: cell, Spot: p, At: p, GivingWay: true}
		if m.facing != (geom.Vec{}) {
			here := m.centre()
			o.Face = geom.NewVec(here.X+m.facing.X*16, here.Y+m.facing.Y*16)
		}
		return o, true
	}
	return MoveOrder{}, false
}

func (k *bodyKeeping) onGoal(self, other body) bool {
	return !apart(k.delta(self.spot, other.at), self.half, other.half, gapFor(self.half))
}

func (k *bodyKeeping) settle(m member, o *MoveOrder, other body) bool {
	k.placeAgain(m, o, append(k.met(o), other))
	return true
}
