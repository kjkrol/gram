package navigation

import (
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

const stallAfter = time.Second

const maxStalls = 5

const asideTime = 400 * time.Millisecond

// how much of the way to its goal a unit keeps while it steps round someone
const asideGoal = 0.5

type openOccupancy struct{}

func (openOccupancy) CanEnter(cell.ID, bool, uid.UID64, cell.Domain) bool { return true }
func (openOccupancy) Enter(cell.ID, bool, uid.UID64, cell.Domain)         {}
func (openOccupancy) Leave(cell.ID, uid.UID64)                            {}
func (openOccupancy) Holders(cell.ID, func(uid.UID64, cell.Domain, bool)) {}
func (openOccupancy) Release(func(uid.UID64) bool)                        {}

type bodyKeeping struct {
	finder  *pathFinder
	grid    grid.Grid
	terrain cell.Terrain
	space   *aabbworld.Space      // nil: flat and open
	ground  func() ground.Heights // nil, or giving nil: level
	index   bodyIndex
}

var _ keeping = (*bodyKeeping)(nil)

func newBodyKeeping(finder *pathFinder, space *aabbworld.Space, ground func() ground.Heights) *bodyKeeping {
	return &bodyKeeping{finder: finder, grid: finder.grid, terrain: finder.terrain, space: space, ground: ground, index: bodyIndex{grid: finder.grid}}
}

func (k *bodyKeeping) occupancy() cell.Occupancy { return k.finder.occupancy }

func (k *bodyKeeping) begin(gather func([]body) []body) {
	k.index.build(gather(k.index.bodies[:0]))
}

// the short way round on a wrapping axis
func (k *bodyKeeping) delta(a, b geom.Vec) geom.Vec {
	if k.space == nil {
		return geom.NewVec(b.X-a.X, b.Y-a.Y)
	}
	w, h, e := k.space.Bounds()
	return geom.NewVec(shortestAxisDelta(a.X, b.X, w, e.WrapsX()), shortestAxisDelta(a.Y, b.Y, h, e.WrapsY()))
}

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

func half(m member) geom.Vec { return geom.NewVec(m.pos.Size.X/2, m.pos.Size.Y/2) }

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
	spots, _ := k.place(group, at, cmd.Cell, cmd.Level)
	for i, p := range spots {
		m := group[i]
		if !p.ok {
			continue
		}
		if cmd.Append && m.order != nil {
			m.order.Enqueue(Goal{Cell: p.cell, Spot: p.spot, At: at, Level: p.level})
			continue
		}
		path, ok := Path{}, m.from == p.cell && k.finder.on(m, p.cell, p.level)
		if !ok {
			path, ok = k.finder.findPath(m.id, m.domain, m.z, m.from, p.cell, p.level)
		}
		if ok {
			issue(m, MoveOrder{Target: p.cell, Level: p.level, Spot: p.spot, At: at, Path: path,
				Leg: m.leg})
		}
	}
}

func (k *bodyKeeping) on(m member, c cell.ID, level Level) bool {
	return k.finder.on(m, c, level)
}

func (k *bodyKeeping) reaches(m member, c cell.ID) bool {
	_, ok := k.finder.findPath(m.id, m.domain, m.z, m.from, c, EitherLevel)
	return ok
}

func (k *bodyKeeping) look(m member, at geom.Vec) MoveOrder {
	here := m.centre()
	stop := here
	if v := math.Hypot(m.vel.X, m.vel.Y); v > 0 && m.brake > 0 {
		d := v / (2 * m.brake)
		stop = k.fold(geom.NewVec(here.X+m.vel.X*d, here.Y+m.vel.Y*d))
	}
	if !k.fits(m, stop, m.z, nil) {
		stop = here
	}
	cell, ok := cellOf(k.grid, k.terrain, stop, &m.z)
	if !ok {
		cell = m.cell
	}
	return MoveOrder{Target: cell, Spot: stop, At: stop, Face: at}
}

func (k *bodyKeeping) ready(m member, o *MoveOrder) {
	if o.Spot != (geom.Vec{}) {
		return
	}
	k.placeAgain(m, o, nil)
}

func (k *bodyKeeping) placeAgain(m member, o *MoveOrder, struck []body) {
	at := o.At
	if at == (geom.Vec{}) {
		at = k.grid.CellCenter(o.Target)
	}
	spots, _ := k.placeClearOf([]member{m}, at, o.Target, o.Level, struck)
	p := spots[0]
	if !p.ok {
		p = placed{spot: k.grid.CellCenter(o.Target), cell: o.Target, level: o.Level}
	}
	if p.cell != o.Target || p.level != o.Level {
		o.Path, o.Leg = Path{}, Leg{}
	}
	o.Target, o.Spot, o.At, o.Level = p.cell, p.spot, at, p.level
}

func (k *bodyKeeping) route(m member, from cell.ID, o *MoveOrder) (Path, bool) {
	if o.Avoids > 0 {
		known := o.Avoid[:o.Avoids]
		blocked := func(c cell.ID) bool { return c != o.Target && slices.Contains(known, c) }
		if p, ok := k.finder.findPathAround(m.id, m.domain, m.z, from, o.Target, o.Level,
			blocked); ok {
			return p, true
		}
	}
	return k.finder.findPath(m.id, m.domain, m.z, from, o.Target, o.Level)
}

func (k *bodyKeeping) lost(m member, from cell.ID, o *MoveOrder, waited time.Duration) (cell.ID, Path, bool, bool) {
	if waited < targetWaitTimeout {
		return 0, Path{}, true, false
	}
	for _, c := range k.tilesRound(o.Target, m.domain, m.domain) {
		if c == from {
			o.Level = EitherLevel
			return c, Path{}, false, true
		}
		if p, ok := k.finder.findPath(m.id, m.domain, m.z, from, c, EitherLevel); ok {
			o.Spot, o.Level = geom.Vec{}, EitherLevel
			return c, p, false, true
		}
	}
	return 0, Path{}, false, false
}

// while m steps round someone it struck it never leans back into them, and still facing into
// them it turns on the spot
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

func (k *bodyKeeping) watch(m member, o *MoveOrder, toward cell.ID, want geom.Vec, d time.Duration) {
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

// going round again one it knew stands there, the last step round over, is no headway: a stall
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

func (k *bodyKeeping) pass(m member, o *MoveOrder, other body) {
	if n := k.wayOff(m, other); n != (geom.Vec{}) {
		k.stepRound(m, o, n)
	}
}

// square off the side of other m is least into, as collision parts two boxes; zero on top of
// each other
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

// head on, to the left of the way off, so two meeting head on pass
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

func (k *bodyKeeping) takes(m member, dir geom.Vec) bool {
	h := half(m)
	reach := 2*max(h.X, h.Y) + gapFor(h)
	c := m.centre()
	at := k.fold(geom.NewVec(c.X+dir.X*reach, c.Y+dir.Y*reach))
	ok := true
	k.grid.CellsUnder(boxAt(at, h), func(cell cell.ID) {
		ok = ok && k.terrain.Kind(cell).Admits(m.domain) &&
			k.finder.climb(m.cell, cell, m.domain) <= yieldClimb && k.finder.steps(m, cell)
	})
	for _, s := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if _, in := k.grid.CellAt(k.fold(geom.NewVec(at.X+s[0]*h.X, at.Y+s[1]*h.Y))); !in {
			ok = false
		}
	}
	return ok
}

func (k *bodyKeeping) met(o *MoveOrder) []body {
	var out []body
	for _, id := range o.Met[:o.Mets] {
		if b, ok := k.index.of(id); ok {
			out = append(out, b)
		}
	}
	return out
}

func (k *bodyKeeping) goalOf(o *MoveOrder) geom.Vec {
	if o.Spot != (geom.Vec{}) {
		return o.Spot
	}
	return k.grid.CellCenter(o.Target)
}

const yieldLinger = time.Second

// never called: the open occupancy refuses nobody
func (k *bodyKeeping) blocked(member, *MoveOrder, cell.ID, bool, time.Duration) (uid.UID64,
	bool) {
	return 0, false
}

func (k *bodyKeeping) byContact() bool { return true }

func (k *bodyKeeping) within(m member) float64 {
	return min(arrivalEpsilon, min(m.pos.Size.X, m.pos.Size.Y)/4)
}

func (k *bodyKeeping) mayStep(m member, _ cell.ID, _ bool, heading geom.Vec) bool {
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

func (k *bodyKeeping) other(id uid.UID64) (body, bool) { return k.index.of(id) }

// square to the line between them, the side m stands on first
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

func (k *bodyKeeping) open(m member, p geom.Vec) (ground bool, in uid.UID64, taken bool) {
	cell, ok := cellOf(k.grid, k.terrain, p, &m.z)
	if !ok || !k.fits(m, p, m.z, nil) || k.finder.climb(m.cell, cell, m.domain) > yieldClimb ||
		!k.finder.steps(m, cell) {
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

func (k *bodyKeeping) stepAside(m member, of body) (MoveOrder, bool) {
	for _, p := range k.asides(m, of) {
		if ground, _, taken := k.open(m, p); !ground || taken {
			continue
		}
		cell, _ := cellOf(k.grid, k.terrain, p, &m.z)
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
