package spacing

import (
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/uid"
)

// StallAfter is how long a unit comes no nearer its next cell before it counts as held.
const StallAfter = time.Second

const maxStalls = 5

const asideTime = 400 * time.Millisecond

// how much of the way to its goal a unit keeps while it steps round someone
const asideGoal = 0.5

// OpenOccupancy holds no cell for anyone: a search blind to the others.
type OpenOccupancy struct{}

func (OpenOccupancy) CanEnter(cell.ID, bool, uid.UID64, cell.Domain) bool { return true }
func (OpenOccupancy) Enter(cell.ID, bool, uid.UID64, cell.Domain)         {}
func (OpenOccupancy) Leave(cell.ID, uid.UID64)                            {}
func (OpenOccupancy) Holders(cell.ID, func(uid.UID64, cell.Domain, bool)) {}
func (OpenOccupancy) Release(func(uid.UID64) bool)                        {}

// BodyKeeping keeps units apart by their boxes, several standing in one cell (BodySpacing).
type BodyKeeping struct {
	finder  *Finder
	grid    grid.Grid
	terrain cell.Terrain
	space   *aabbworld.Space      // nil: flat and open
	ground  func() ground.Heights // nil, or giving nil: level
	index   bodies.Index
	avoid   avoiding
	// the one takes asks about and whether every cell so far takes it
	taker bodies.Member
	taken bool
	take  func(cell.ID) // the method value, bound once
}

var _ Keeping = (*BodyKeeping)(nil)

// NewBodyKeeping keeps the bodies apart over finder's board and space, on ground's heights.
func NewBodyKeeping(finder *Finder, space *aabbworld.Space, ground func() ground.Heights) *BodyKeeping {
	k := &BodyKeeping{
		finder:  finder,
		grid:    finder.grid,
		terrain: finder.Terrain,
		space:   space,
		ground:  ground,
		index:   bodies.NewIndex(finder.grid),
	}
	k.take = k.takeCell
	k.avoid.bind(finder, &Answers{})
	return k
}

// WithAnswers has k keep its searches' answers in a, which other keepings may share.
func (k *BodyKeeping) WithAnswers(a *Answers) *BodyKeeping {
	k.avoid.bind(k.finder, a)
	return k
}

func (k *BodyKeeping) Occupancy() cell.Occupancy { return k.finder.Occupancy }

func (k *BodyKeeping) Begin(gather func([]bodies.Body) []bodies.Body) {
	k.index.Build(gather(k.index.Bodies[:0]))
}

// the short way round on a wrapping axis
func (k *BodyKeeping) delta(a, b geom.Vec) geom.Vec {
	if k.space == nil {
		return geom.NewVec(b.X-a.X, b.Y-a.Y)
	}
	w, h, e := k.space.Bounds()
	return geom.NewVec(ShortestAxisDelta(a.X, b.X, w, e.WrapsX()), ShortestAxisDelta(a.Y, b.Y, h, e.WrapsY()))
}

func (k *BodyKeeping) fold(p geom.Vec) geom.Vec {
	if k.space == nil {
		return p
	}
	w, h, e := k.space.Bounds()
	if e.WrapsX() {
		p = geom.NewVec(wrapAxis(p.X, float32(w)), p.Y)
	}
	if e.WrapsY() {
		p = geom.NewVec(p.X, wrapAxis(p.Y, float32(h)))
	}
	return p
}

func wrapAxis(v, size float32) float32 {
	v = f32.Mod(v, size)
	if v < 0 {
		v += size
	}
	return v
}

func half(m bodies.Member) geom.Vec { return geom.NewVec(m.Pos.Size.X/2, m.Pos.Size.Y/2) }

func (k *BodyKeeping) Orders(members []bodies.Member, cmd order.MoveTo, issue func(bodies.Member,
	order.MoveOrder)) {
	at := cmd.At
	if at == (geom.Vec{}) {
		at = k.grid.CellCenter(cmd.Cell)
	}
	spots, _ := k.place(members, at, cmd.Cell, cmd.Level)
	for i, p := range spots {
		m := members[i]
		if !p.ok {
			continue
		}
		if cmd.Append && m.Order != nil {
			m.Order.Enqueue(order.Goal{Cell: p.cell, Spot: p.spot, At: at, Level: p.level})
			continue
		}
		issue(m, order.MoveOrder{Target: p.cell, Level: p.level, Spot: p.spot, At: at, Leg: m.Leg})
	}
}

func (k *BodyKeeping) On(m bodies.Member, c cell.ID, level order.Level) bool {
	return k.finder.on(m, c, level)
}

func (k *BodyKeeping) Look(m bodies.Member, at geom.Vec) order.MoveOrder {
	here := m.Centre()
	stop := here
	if v := f32.Hypot(m.Vel.X, m.Vel.Y); v > 0 && m.Brake > 0 {
		d := v / (2 * m.Brake)
		stop = k.fold(geom.NewVec(here.X+m.Vel.X*d, here.Y+m.Vel.Y*d))
	}
	if !k.fits(m, stop, m.Z, nil) {
		stop = here
	}
	cell, ok := CellOf(k.grid, k.terrain, stop, &m.Z)
	if !ok {
		cell = m.Cell
	}
	return order.MoveOrder{Target: cell, Spot: stop, At: stop, Face: at}
}

func (k *BodyKeeping) Ready(m bodies.Member, o *order.MoveOrder) {
	if o.Spot != (geom.Vec{}) {
		return
	}
	k.placeAgain(m, o, nil)
}

func (k *BodyKeeping) placeAgain(m bodies.Member, o *order.MoveOrder, struck []bodies.Body) {
	at := o.At
	if at == (geom.Vec{}) {
		at = k.grid.CellCenter(o.Target)
	}
	spots, _ := k.placeClearOf([]bodies.Member{m}, at, o.Target, o.Level, struck)
	p := spots[0]
	if !p.ok {
		p = placed{spot: k.grid.CellCenter(o.Target), cell: o.Target, level: o.Level}
	}
	if p.cell != o.Target || p.level != o.Level {
		o.Path, o.Leg = order.Path{}, order.Leg{}
		order.Found(o)
	}
	o.Target, o.Spot, o.At, o.Level = p.cell, p.spot, at, p.level
}

func (k *BodyKeeping) Route(m bodies.Member, from cell.ID, o *order.MoveOrder) (order.Path, bool) {
	return k.avoid.route(k.finder, m, from, o)
}

func (k *BodyKeeping) Lost(m bodies.Member, from cell.ID, o *order.MoveOrder, waited time.Duration) (cell.ID, order.Path, bool, bool) {
	if waited < TargetWaitTimeout {
		return 0, order.Path{}, true, false
	}
	for _, c := range k.finder.tilesRound(o.Target, m.Domain, m.Domain) {
		if c == from {
			o.Level = order.EitherLevel
			return c, order.Path{}, false, true
		}
		if p, ok := k.finder.FindPath(m.ID, m.Domain, m.Body(), from, c, order.EitherLevel); ok {
			o.Spot, o.Level = geom.Vec{}, order.EitherLevel
			return c, p, false, true
		}
	}
	return 0, order.Path{}, false, false
}

// while m steps round someone it struck, or touches someone, it never moves into them
func (k *BodyKeeping) Steer(m bodies.Member, st steering.Helm, dir geom.Vec, speed float32) {
	if o := m.Order; o != nil && o.AsideFor > 0 {
		if o.Aside == (geom.Vec{}) {
			speed = 0
		}
		d := o.Aside
		if n := f32.Hypot(dir.X, dir.Y); n > 0 {
			d = geom.NewVec(d.X+dir.X/n*asideGoal, d.Y+dir.Y/n*asideGoal)
		}
		if into := -(d.X*o.Struck.X + d.Y*o.Struck.Y); into > 0 {
			d = geom.NewVec(d.X+o.Struck.X*into, d.Y+o.Struck.Y*into)
		}
		n := f32.Hypot(d.X, d.Y)
		if n > f32.Eps(asideGoal) && k.takes(m, geom.NewVec(d.X/n, d.Y/n)) {
			dir = geom.NewVec(d.X/n, d.Y/n)
		}
		dir, speed = clearOf(m.Facing, dir, speed, o.Struck)
	}
	if m.Touching.X != 0 {
		dir, speed = clearOf(m.Facing, dir, speed, geom.NewVec(m.Touching.X, 0))
	}
	if m.Touching.Y != 0 {
		dir, speed = clearOf(m.Facing, dir, speed, geom.NewVec(0, m.Touching.Y))
	}
	if dir != (geom.Vec{}) {
		st.Request(dir)
	}
	st.RequestSpeed(speed)
}

// the way and speed that keep clear of one touched, n leading off them: facing into them it turns
// on the spot, a way straight through them is none, and a turn that would swing it through them
// sets off round their far side
func clearOf(facing, dir geom.Vec, speed float32, n geom.Vec) (geom.Vec, float32) {
	facingInto := facing.X*n.X+facing.Y*n.Y < -0.01
	if facingInto {
		speed = 0
	}
	l := f32.Hypot(dir.X, dir.Y)
	if l == 0 {
		return dir, speed
	}
	if into := -(dir.X*n.X + dir.Y*n.Y); into > 0 {
		dir = geom.NewVec(dir.X+n.X*into, dir.Y+n.Y*into)
	}
	along := f32.Hypot(dir.X, dir.Y)
	if along < 0.01*l {
		return geom.Vec{}, 0
	}
	dir = geom.NewVec(dir.X/along, dir.Y/along)
	if facingInto || facing == (geom.Vec{}) {
		return dir, speed
	}
	// the steering turns the short way, which half round runs through them
	mid := geom.NewVec(facing.X+dir.X, facing.Y+dir.Y)
	if w := f32.Hypot(mid.X, mid.Y); w < 1e-3 || mid.X*n.X+mid.Y*n.Y < -0.01*w {
		return n, speed
	}
	return dir, speed
}

func (k *BodyKeeping) Watch(m bodies.Member, o *order.MoveOrder, toward cell.ID, want geom.Vec, d time.Duration) {
	if o.AsideFor = max(o.AsideFor-d, 0); o.AsideFor == 0 {
		o.Aside, o.Struck = geom.Vec{}, geom.Vec{}
	}
	c := m.Centre()
	dist := f32.Hypot(want.X-c.X, want.Y-c.Y)
	if m.Cell == o.Toward && toward != o.Toward {
		o.Stalls = 0
	}
	if toward != o.Toward || o.Closest == 0 && o.Stalled == 0 {
		o.Toward, o.Closest, o.Stalled = toward, dist, 0
		return
	}
	if dist < o.Closest-min(m.Pos.Size.X, m.Pos.Size.Y)/4 {
		o.Closest, o.Stalled = dist, 0
		return
	}
	if o.Stalled += d; o.Stalled >= StallAfter {
		o.Bumped, o.Stalled, o.Struck, o.Hit, o.HitUnit, o.AsideFor = true, 0, geom.Vec{}, 0, false, 0
	}
}

func (k *BodyKeeping) Bump(m bodies.Member, o *order.MoveOrder) BumpAnswer {
	if o.Struck == (geom.Vec{}) {
		if o.Stalls++; o.Stalls > maxStalls {
			return GiveUp
		}
		o.Path, o.Leg, o.Closest = order.Path{}, order.Leg{}, f32.Inf(1)
		return CarryOn
	}
	k.stepRound(m, o, o.Struck)
	return CarryOn
}

// going round again one it knew stands there, the last step round over, is no headway: a stall
func (k *BodyKeeping) Detour(m bodies.Member, o *order.MoveOrder, other bodies.Body) BumpAnswer {
	if !other.Moving && other.Domain&m.Domain != 0 {
		if slices.Contains(o.Met[:o.Mets], other.ID) && o.AsideFor == 0 {
			if o.Stalls++; o.Stalls > maxStalls {
				return GiveUp
			}
		}
		order.Meet(o, other.ID)
		if c, ok := k.grid.CellAt(other.At); ok && c != o.Target && c != m.Cell {
			order.Learn(o, c)
			if o.Path.Index < o.Path.Length && slices.Contains(o.Path.Steps[o.Path.Index:o.Path.Length], c) || o.Leg.Active && o.Leg.To == c {
				o.Path, o.Leg = order.Path{}, order.Leg{}
			}
		}
	}
	if n := k.wayOff(m, other); n != (geom.Vec{}) {
		k.stepRound(m, o, n)
	}
	return CarryOn
}

func (k *BodyKeeping) Pass(m bodies.Member, o *order.MoveOrder, other bodies.Body) {
	if n := k.wayOff(m, other); n != (geom.Vec{}) {
		k.stepRound(m, o, n)
	}
}

// square off the side of other m is least into, as collision parts two boxes; zero on top of
// each other
func (k *BodyKeeping) wayOff(m bodies.Member, other bodies.Body) geom.Vec {
	d := k.delta(other.At, m.Centre())
	h := half(m)
	intoX := h.X + other.Half.X - f32.Abs(d.X)
	intoY := h.Y + other.Half.Y - f32.Abs(d.Y)
	switch {
	case d == (geom.Vec{}):
		return geom.Vec{}
	case intoX < intoY:
		return geom.NewVec(f32.Copysign(1, d.X), 0)
	default:
		return geom.NewVec(0, f32.Copysign(1, d.Y))
	}
}

// head on, to the left of the way off, so two meeting head on pass
func (k *BodyKeeping) stepRound(m bodies.Member, o *order.MoveOrder, n geom.Vec) {
	at := m.Centre()
	goal := k.delta(at, k.goalOf(o))
	aside := geom.NewVec(-n.Y, n.X)
	l := f32.Hypot(goal.X, goal.Y)
	if l > f32.Eps(max(f32.Abs(at.X), f32.Abs(at.Y))) && (aside.X*goal.X+aside.Y*goal.Y)/l < -0.2 {
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

func (k *BodyKeeping) takes(m bodies.Member, dir geom.Vec) bool {
	h := half(m)
	reach := 2*max(h.X, h.Y) + gapFor(h)
	c := m.Centre()
	at := k.fold(geom.NewVec(c.X+dir.X*reach, c.Y+dir.Y*reach))
	k.taker, k.taken = m, true
	k.grid.CellsUnder(bodies.BoxAt(at, h), k.take)
	ok := k.taken
	for _, s := range [4][2]float32{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if _, in := k.grid.CellAt(k.fold(geom.NewVec(at.X+s[0]*h.X, at.Y+s[1]*h.Y))); !in {
			ok = false
		}
	}
	return ok
}

func (k *BodyKeeping) takeCell(c cell.ID) {
	m := &k.taker
	k.taken = k.taken && k.terrain.Kind(c).Admits(m.Domain) &&
		k.finder.climb(m.Cell, c, m.Domain) <= YieldClimb && k.finder.steps(*m, c)
}

func (k *BodyKeeping) met(o *order.MoveOrder) []bodies.Body {
	var out []bodies.Body
	for _, id := range o.Met[:o.Mets] {
		if b, ok := k.index.Of(id); ok {
			out = append(out, b)
		}
	}
	return out
}

func (k *BodyKeeping) goalOf(o *order.MoveOrder) geom.Vec {
	if o.Spot != (geom.Vec{}) {
		return o.Spot
	}
	return k.grid.CellCenter(o.Target)
}

// YieldLinger is how long a unit stands where it stepped aside before it goes on.
const YieldLinger = time.Second

// never called: the open occupancy refuses nobody
func (k *BodyKeeping) Blocked(bodies.Member, *order.MoveOrder, cell.ID, bool,
	time.Duration) (uid.UID64, bool) {
	return 0, false
}

func (k *BodyKeeping) ByContact() bool { return true }

func (k *BodyKeeping) Within(m bodies.Member) float32 {
	return min(ArrivalEpsilon, min(m.Pos.Size.X, m.Pos.Size.Y)/4)
}

func (k *BodyKeeping) MayStep(m bodies.Member, _ cell.ID, _ bool, heading geom.Vec,
	near *bodies.Nearby) bool {
	if near == nil {
		near = &k.index.Own
	}
	c, h := m.Centre(), half(m)
	feel := gapFor(h) / 4
	ahead := geom.NewVec(c.X+heading.X*feel, c.Y+heading.Y*feel)
	for _, i := range near.Near(&k.index, bodies.BoxAt(ahead, k.nearHalf(h))) {
		b := &k.index.Bodies[i]
		if b.ID == m.ID || b.Domain&m.Domain == 0 {
			continue
		}
		d := k.delta(c, b.At)
		if d.X*heading.X+d.Y*heading.Y > 0 && !bodies.Apart(k.delta(ahead, b.At), h, b.Half, 0) {
			return false
		}
	}
	return true
}

func (k *BodyKeeping) nearHalf(h geom.Vec) geom.Vec {
	return geom.NewVec(h.X+k.index.Largest, h.Y+k.index.Largest)
}

func (k *BodyKeeping) Other(id uid.UID64) (bodies.Body, bool) { return k.index.Of(id) }

// square to the line between them, the side m stands on first
func (k *BodyKeeping) asides(m bodies.Member, asker bodies.Body) [2]geom.Vec {
	here, h := m.Centre(), half(m)
	way := k.delta(asker.At, here)
	if l := f32.Hypot(way.X, way.Y); l > f32.Eps(max(f32.Abs(here.X), f32.Abs(here.Y))) {
		way = geom.NewVec(way.X/l, way.Y/l)
	} else {
		way = geom.NewVec(1, 0)
	}
	across := geom.NewVec(-way.Y, way.X)
	off := k.delta(asker.At, here)
	side := off.X*across.X + off.Y*across.Y
	sign := float32(1)
	if side < 0 {
		sign = -1
	}
	clear := spanAcross(asker.Half, across) + spanAcross(h, across) + gapFor(h)/4
	var out [2]geom.Vec
	for i, step := range [2]float32{sign * (clear - f32.Abs(side)), -sign * (clear + f32.Abs(side))} {
		if f32.Abs(step) < gapFor(h)/4 {
			step = f32.Copysign(gapFor(h)/4, step)
		}
		out[i] = k.fold(geom.NewVec(here.X+across.X*step, here.Y+across.Y*step))
	}
	return out
}

func spanAcross(h, a geom.Vec) float32 { return h.X*f32.Abs(a.X) + h.Y*f32.Abs(a.Y) }

func (k *BodyKeeping) open(m bodies.Member, p geom.Vec) (ground bool, in uid.UID64, taken bool) {
	cell, ok := CellOf(k.grid, k.terrain, p, &m.Z)
	if !ok || !k.fits(m, p, m.Z, nil) || k.finder.climb(m.Cell, cell, m.Domain) > YieldClimb ||
		!k.finder.steps(m, cell) {
		return false, 0, false
	}
	h := half(m)
	for _, i := range k.index.Near(bodies.BoxAt(p, k.nearHalf(h))) {
		b := &k.index.Bodies[i]
		if b.ID != m.ID && b.Domain&m.Domain != 0 && !bodies.Apart(k.delta(p, b.At), h, b.Half, 0) {
			return true, b.ID, true
		}
	}
	return true, 0, false
}

func (k *BodyKeeping) StepAside(m bodies.Member, of bodies.Body) (order.MoveOrder, bool) {
	for _, p := range k.asides(m, of) {
		if ground, _, taken := k.open(m, p); !ground || taken {
			continue
		}
		cell, _ := CellOf(k.grid, k.terrain, p, &m.Z)
		o := order.MoveOrder{Target: cell, Spot: p, At: p, GivingWay: true}
		if m.Facing != (geom.Vec{}) {
			here := m.Centre()
			o.Face = geom.NewVec(here.X+m.Facing.X*16, here.Y+m.Facing.Y*16)
		}
		return o, true
	}
	return order.MoveOrder{}, false
}

func (k *BodyKeeping) OnGoal(self, other bodies.Body) bool {
	return !bodies.Apart(k.delta(self.Spot, other.At), self.Half, other.Half, gapFor(self.Half))
}

func (k *BodyKeeping) Settle(m bodies.Member, o *order.MoveOrder, other bodies.Body) bool {
	k.placeAgain(m, o, append(k.met(o), other))
	return true
}
