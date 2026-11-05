package traffic

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// Abstracts counts the units come into the far mode, once an entry, since the system was made.
func (s *System) Abstracts() uint64 { return s.abstracts }

// Snaps counts the units come down from the far mode onto a cell other than the one under them
// since the system was made.
func (s *System) Snaps() uint64 { return s.snaps }

// headFar brings w's order up to where w stands in the far mode, off the cells: asking the rest
// of its route where the one it has is spent short of its goal or broken under it
func (s *System) headFar(w *walker, changed bool, d time.Duration) stepped {
	o, p := w.o, &w.o.Path
	o.Kept, o.Bumped, o.Holding, o.TrailFor = false, false, 0, 0
	o.Retry = max(o.Retry-d, 0)
	w.at.Next = 0
	if w.actual != w.at.Cell {
		s.moveTo(w, w.actual)
	}
	w.m.Cell, w.m.From, w.m.Leg = w.at.Cell, w.at.Cell, order.Leg{}
	if t, ok := s.told[w.id]; ok && t.stop {
		w.st.RequestSpeed(0)
		s.arrive(w)
		return stood
	}
	if changed && p.Index < p.Length && !s.admitsAll(p.Steps[p.Index:p.Length], w.domain) {
		p.Length = 0
	}
	s.keep.Ready(w.m, o)
	if p.Index < p.Length || w.at.Cell == o.Target {
		return onward
	}
	// waiting on a search that found nothing, as on cells
	if o.Waited == 0 || o.Retry == 0 {
		o.Ask = order.AskRoute
		return asking
	}
	w.st.RequestSpeed(0)
	if o.Waited += d; o.Waited < spacing.TargetWaitTimeout {
		return stood
	}
	o.Ask = order.AskNearest
	return asking
}

// tailFar drives w along its route in the far mode, from the middle of one of its cells to the
// next, each step at the share of its top speed the planner prices it at, through whatever stands
// there, till it stands at its goal
func (s *System) tailFar(w *walker, d time.Duration) {
	o, st, p := w.o, w.st, &w.o.Path
	have := w.base.Pos.Center()
	reach := lookaheadReach(st, float32(d.Seconds()))
	target := o.Target
	for p.Index < p.Length {
		c := p.Steps[p.Index]
		if c == target && !order.Through(p, c) ||
			!passed(have, s.unwrap(have, s.place(c, w.off)), s.unwrap(have,
				s.place(order.At(p), w.off)), reach) {
			break
		}
		p.Index++
	}
	waypoint, final := target, true
	switch {
	case p.Index < p.Length:
		waypoint = p.Steps[p.Index]
		final = waypoint == target && !order.Through(p, waypoint)
	case p.Length > 0 && p.Steps[p.Length-1] != target:
		return // spent short of its goal: the rest asked the next step, its way kept meanwhile
	}
	end := s.goal(target, o.Spot, w.off)
	want := s.unwrap(have, s.place(waypoint, w.off))
	if final {
		want = s.unwrap(have, end)
	}
	dx, dy := want.X-have.X, want.Y-have.Y
	dist := f32.Hypot(dx, dy)
	if dist > 0 {
		o.Way = geom.NewVec(dx/dist, dy/dist)
	}
	s.keep.Watch(w.m, o, waypoint, want, d)
	from := have // with no route, at its goal's cell
	if p.Length > 0 {
		from = s.unwrap(have, s.place(order.At(p), w.off))
	}
	speed := st.MaxSpeed * s.stepPace(w)
	switch {
	case !final:
		s.driveFar(w, have, s.ahead(have, want, p, waypoint, target, end, w.off), reach, speed)
	case o.Queued > 0 && passed(have, want, from, reach):
		order.Advance(o)
		s.route = append(s.route[:0], s.unwrap(have, s.goal(o.Target, o.Spot, w.off)))
		s.driveFar(w, have, s.route, reach, speed)
	case o.Queued > 0:
		s.route = append(s.route[:0], want)
		s.driveFar(w, have, s.route, reach, speed)
	default:
		// far off the motion takes several steps in one: past its goal it went too far
		within := s.keep.Within(w.m)
		if dist <= within || passed(have, want, from, 0) {
			s.arriveFar(w, want, geom.NewVec(dx, dy))
			return
		}
		dir := geom.NewVec(dx, dy)
		st.Request(dir)
		st.RequestSpeed(min(speed, approach(st, dist, within)) *
			turnFactor(w.base.Vel.Dir, dir))
	}
}

// driveFar heads w for the point reach along route, at speed slowed by the turn
func (s *System) driveFar(w *walker, have geom.Vec, route []geom.Vec, reach, speed float32) {
	at := lookahead(have, route, reach)
	if at == have {
		w.st.RequestSpeed(speed)
		return
	}
	dir := geom.NewVec(at.X-have.X, at.Y-have.Y)
	w.st.Request(dir)
	w.st.RequestSpeed(speed * turnFactor(w.base.Vel.Dir, dir))
}

// stepPace is the share of its top speed w keeps on the step of its route it is on, as the
// planner prices the step, read once a step: whole where the planner refuses it, past the
// route's end or on a step of no length
func (s *System) stepPace(w *walker) float32 {
	o, p := w.o, &w.o.Path
	if p.Index >= p.Length {
		return 1
	}
	step := [2]cell.ID{order.At(p), p.Steps[p.Index]}
	if o.Pace > 0 && o.Paced == step {
		return o.Pace
	}
	level := cell.OnGround
	if order.Deck(p, p.Index) {
		level = cell.OnDeck
	}
	o.Pace, o.Paced = 1, step
	cost, ok := s.pathFinder.Price(step[0], step[1], s.pathFinder.KindAt(step[1], level), level,
		w.domain)
	if length := s.grid.NeighborCost(step[0], step[1]); ok && cost > 0 && length > 0 {
		o.Pace = min(float32(length/cost), 1)
	}
	return o.Pace
}

// arriveFar has w, by its goal at want, by off from it, stand there, back on the cells
func (s *System) arriveFar(w *walker, want, off geom.Vec) {
	st := w.st
	st.RequestSpeed(0)
	st.Speed = 0
	if face := w.o.Face; face != (geom.Vec{}) {
		st.Request(geom.NewVec(face.X-want.X, face.Y-want.Y))
	}
	if s.space != nil && (f32.Abs(off.X) > f32.Eps(want.X) || f32.Abs(off.Y) > f32.Eps(want.Y)) {
		s.space.Move(&w.base.Pos.AABB, off)
	}
	if at, deck, ok := spacing.LevelOf(s.grid, s.terrain, s.from(want, w.off), &w.m.Z); ok {
		s.occupancy.Enter(at, deck, w.id, w.domain)
		s.moveTo(w, at)
	}
	s.arrive(w)
}

// standFor has w, an aggregate unfolded or ticked finer than L2, stand while its parts walk for
// it, whatever its level till it has folded: come down from the far mode first as a unit comes
// down, its order kept as it is for them to inherit (Inherit), its step in flight let go; it
// holds no cell, being its parts (the board keeps none for it, world.Unfolded)
func (s *System) standFor(w *walker) stepped {
	o := w.o
	w.st.RequestSpeed(0)
	w.at.Next = 0
	if w.far {
		if c, _, _, ok := s.land(w, false); ok {
			s.moveTo(w, c)
		}
		s.unmark(w)
	}
	if o.Leg.Active {
		s.releaseLeg(o.Leg, w.id)
		s.moveTo(w, w.actual)
		o.Leg = order.Leg{}
	}
	s.occupancy.Leave(w.at.Cell, w.id)
	s.occupancy.Leave(w.actual, w.id)
	if t, ok := s.told[w.id]; ok && t.stop {
		s.arrive(w)
	}
	return stood
}

// goFar takes w off the cells into the far mode: the step it was taking and the cells it held let
// go, marked so the collision and the board leave it be
func (s *System) goFar(w *walker) {
	o := w.o
	if o.Leg.Active {
		s.releaseLeg(o.Leg, w.id)
		o.Leg = order.Leg{}
	}
	w.at.Next = 0
	if w.far {
		return
	}
	s.occupancy.Leave(w.at.Cell, w.id)
	s.occupancy.Leave(w.actual, w.id)
	s.mark(w)
	s.abstracts++
}

// descend brings w down from the far mode, its sector ticked finer than L2, for the head to go on
// on the cells: onto the cell under it, its route walked on from there where that is one of its
// cells round its next step, else asked anew; onto the nearest free cell taking it where that one
// is taken or does not, a route asked from there; with none near, asking the nearest free cell
// round its goal
func (s *System) descend(w *walker) stepped {
	o, p := w.o, &w.o.Path
	w.at.Next = 0
	s.unmark(w)
	c, deck, snapped, ok := s.land(w, true)
	if !ok {
		p.Length, p.Index = 0, 0
		o.Waited = max(o.Waited, spacing.TargetWaitTimeout)
		o.Ask = order.AskNearest
		return asking
	}
	s.occupancy.Enter(c, deck, w.id, w.domain)
	s.moveTo(w, c)
	w.current, w.actual, w.onDeck = c, c, deck
	if snapped || !s.anchor(p, c, deck) {
		p.Length, p.Index = 0, 0
	}
	return onward
}

// land has w, come down from the far mode, on the cell under it where that takes it and, held,
// is free, else moves it onto the nearest such: the cell, whether on a raised deck and whether w
// was snapped off the one under it; false with none near, w left where it is
func (s *System) land(w *walker, held bool) (c cell.ID, deck, snapped, ok bool) {
	c, deck = w.actual, w.onDeck
	if s.terrain.Kind(c).Admits(w.domain) &&
		(!held || s.occupancy.CanEnter(c, deck, w.id, w.domain)) {
		return c, deck, false, true
	}
	taken := s.taken
	if !held {
		taken = nil
	}
	s.taker, s.takerIn = w.id, w.domain
	snap, found := s.pathFinder.Snap(w.domain, w.m.Body(), c, taken)
	if !found {
		return c, deck, false, false
	}
	have := w.base.Pos.Center()
	to := s.unwrap(have, s.place(snap, w.off))
	if s.space != nil {
		s.space.Move(&w.base.Pos.AABB, geom.NewVec(to.X-have.X, to.Y-have.Y))
	}
	deck = false
	if at, on, ok := spacing.LevelOf(s.grid, s.terrain, s.from(to, w.off), &w.m.Z); ok &&
		at == snap {
		deck = on
	}
	s.snaps++
	return snap, deck, true, true
}

// anchor has p's route stand at c, on a raised deck where deck says, where c is a cell of it
// round its next steps: the one it stands at, one of the next few or a neighbour of the next;
// false where c is none of them or the route is spent
func (s *System) anchor(p *order.Path, c cell.ID, deck bool) bool {
	if p.Index >= p.Length {
		return false
	}
	if c == order.At(p) {
		return true
	}
	for k := p.Index; k < min(p.Index+maxAhead, p.Length); k++ {
		if p.Steps[k] == c {
			p.Index = k + 1
			return true
		}
	}
	next := p.Steps[p.Index]
	if _, ok := grid.Way(s.grid, c, next); !ok {
		return false
	}
	if p.Index == 0 {
		p.From = c
		return true
	}
	k := p.Index - 1
	p.Steps[k] = c
	if p.Decks &^= 1 << k; deck {
		p.Decks |= 1 << k
	}
	return true
}

func (s *System) isTaken(c cell.ID, deck bool) bool {
	return !s.occupancy.CanEnter(c, deck, s.taker, s.takerIn)
}

func (s *System) mark(w *walker) {
	if !w.far {
		w.far = true
		s.toggled(w.id, true)
	}
}

func (s *System) unmark(w *walker) {
	if w.far {
		w.far = false
		s.toggled(w.id, false)
	}
}

func (s *System) toggled(id uid.UID64, on bool) {
	s.abstracting = append(s.abstracting, toggled{id: id, on: on})
	if s.marked == nil {
		s.marked = map[uid.UID64]bool{}
	}
	s.marked[id] = on
}

// markedNow says whether id carries the mark as the Update has left it so far, had it as the
// Update began
func (s *System) markedNow(id uid.UID64, had bool) bool {
	if on, ok := s.marked[id]; ok {
		return on
	}
	return had
}

// toggle puts on and takes off the marks of the Update, in order, once its chunks are done: they
// move the entity
func (s *System) toggle(cb *goke.CmdBuf) {
	for _, t := range s.abstracting {
		if t.on {
			cb.AddOne(t.id, s.abstractID, sector.Abstract{})
		} else {
			cb.RemoveCompOne(t.id, s.abstractID)
		}
	}
	s.abstracting = s.abstracting[:0]
	clear(s.marked)
}
