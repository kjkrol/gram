package traffic

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/store"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// System moves the units along their orders' routes, step by step, among the others.
type System struct {
	Pace       goke.Opt
	grid       grid.Grid
	terrain    cell.Terrain
	occupancy  cell.Occupancy
	space      *aabbworld.Space
	pathFinder *spacing.Finder
	keep       spacing.Keeping
	gathering  func([]bodies.Body) []bodies.Body // the method value, bound once
	finished   []uid.UID64                       // a chunk's ids whose order is over
	// who came at each unit last tick and this tick, swapped at the tick's end, so two coming at
	// each other's cells know it whatever order the chunks come in
	wanted, wanting map[uid.UID64]uid.UID64

	query   *goke.Query
	outside *goke.Query // a fork of query, for what is asked between the system's Updates
	cell    goke.Comp[unit.At]
	base    goke.Comp[world.Base]
	steer   goke.Comp[steering.Steering]
	course  goke.Comp[steering.Course]
	order   goke.OptComp[order.MoveOrder]
	mover   goke.OptComp[unit.Mover]
	z       goke.OptComp[world.Z]
	coll    goke.OptComp[collision.Collider]
	hand    goke.OptComp[steering.Driven]
	pace    goke.OptComp[steering.Pace]
	route   []geom.Vec

	owners      goke.OptComp[tag.Tags[owner.Family]]
	lastOrder   goke.OptComp[order.LastOrder]
	lastOrderID goke.CompID
	lastLacking []order.LastOrder
	lastIDs     []uid.UID64
	mind        goke.OptComp[store.Mind]
	blocked     goke.OptComp[order.Blocked]
	arrived     goke.OptComp[order.Arrived]
	blockedID   goke.CompID
	arrivedID   goke.CompID
	blocking    map[uid.UID64]order.Blocked
	arrivals    []arrival

	Given     *GivenQueues
	told      map[uid.UID64]told
	touched   []touching
	felt      map[[2]uid.UID64]bool // the pairs touching this tick, each once; refused, one way
	Touches   *host.PairRules[order.Touch]
	marksCols *cgoke.Columns
	Bumps     Bumps
	marks     *goke.Query
	markCell  goke.Comp[unit.At]
	Tick      host.TickSource

	orderID goke.CompID

	arrivedEditor *goke.Editor

	states   goke.OptComp[tag.Tags[unit.States]]
	statesID goke.CompID
	lacking  []uid.UID64 // the ids whose chunk had no States family, to get it

	// Routing answers the asks of the orders (MoveOrder.Ask) between an Update's two passes
	Routing goke.System
	asked   []uid.UID64 // the units that asked, in the first pass
	askedIn []bool      // whether each had entered a cell before it asked

	terrainSeen uint64
}

var _ goke.System = (*System)(nil)

const bumpInterval = 500 * time.Millisecond

// NewSystem moves the units over grid's terrain, holding their steps in occupancy.
func NewSystem(pathFinder *spacing.Finder, grid grid.Grid, terrain cell.Terrain, occupancy cell.Occupancy) *System {
	s := &System{
		grid:       grid,
		terrain:    terrain,
		occupancy:  occupancy,
		pathFinder: pathFinder,
		keep:       spacing.NewCellKeeping(pathFinder, occupancy),
	}
	s.gathering = s.gather
	return s
}

// WithKeeping has k keep the units apart.
func (s *System) WithKeeping(k spacing.Keeping) *System {
	s.keep, s.occupancy = k, k.Occupancy()
	return s
}

// BindSpace has the units found in space.
func (s *System) BindSpace(space *aabbworld.Space) { s.space = space }

func (s *System) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.cell, &s.base, &s.steer, &s.course).
		Optional(&s.order).
		Optional(&s.mover).
		Optional(&s.z).
		Optional(&s.coll).
		Optional(&s.hand).
		Optional(&s.pace).
		Optional(&s.owners, &s.lastOrder).
		Optional(&s.mind, &s.blocked, &s.arrived).
		Optional(&s.states)
	if s.Pace != nil {
		qb.With(s.Pace)
	}
	s.query = qb.Build()
	s.outside = s.query.Fork()
	s.orderID = si.RegComp[order.MoveOrder]()
	s.blockedID, s.arrivedID, s.lastOrderID = si.RegComp[order.Blocked](), si.RegComp[order.Arrived](), si.RegComp[order.LastOrder]()
	s.arrivedEditor = s.query.NewEditorBuilder().Remove(goke.Remove[order.MoveOrder]()).Build()
	s.statesID = si.RegComp[tag.Tags[unit.States]]()
	if s.Routing != nil {
		s.Routing.Init(si)
	}
	s.seedLegs()
	if s.Touches != nil {
		marks := si.NewQueryBuilder(&s.markCell)
		s.marksCols = cgoke.NewColumns(marks)
		s.Touches.Bind(s.marksCols)
		s.marks = marks.Build()
	}
}

func (s *System) Update(cb *goke.CmdBuf, d time.Duration) {
	changed := false
	if v, ok := s.terrain.(interface{ Version() uint64 }); ok && v.Version() != s.terrainSeen {
		s.terrainSeen, changed = v.Version(), true
	}
	s.keep.Begin(s.gathering)
	s.bump()
	s.touched = s.touched[:0]
	if s.felt == nil {
		s.felt = map[[2]uid.UID64]bool{}
	}
	if s.keep.ByContact() {
		// the rules' commands for the boxes that met are carried out this very tick
		s.feel()
		s.touch(cb, d)
		s.touched = s.touched[:0]
	}
	if s.Given != nil {
		if s.told == nil {
			s.told = map[uid.UID64]told{}
		}
		s.Given.drain(s.told)
	}

	s.asked, s.askedIn = s.asked[:0], s.askedIn[:0]
	for s.query.Paced(d); s.query.Next(); {
		cursor := s.query.Cursor()
		orders := s.order.Slice(cursor)
		if orders == nil {
			s.standing(cb, cursor)
			continue
		}

		cells, bases, states := s.cell.Slice(cursor), s.base.Slice(cursor), s.states.Slice(cursor)
		steers, courses := s.steer.Slice(cursor), s.course.Slice(cursor)
		movers, zs := s.mover.Slice(cursor), s.z.Slice(cursor)
		minds, owned := s.mind.Slice(cursor), s.owners.Slice(cursor)
		arrived, lasts := s.arrived.Slice(cursor), s.lastOrder.Slice(cursor)
		hands, colls, paces := s.hand.Slice(cursor), s.coll.Slice(cursor), s.pace.Slice(cursor)
		arrivedIDs := s.finished[:0]

		for i, id := range cursor.IDs {
			w := walker{id: id, o: &orders[i], at: &cells[i], base: &bases[i],
				st:    steering.Helm{Steering: &steers[i], Course: &courses[i]},
				state: ref(states, i), last: ref(lasts, i), minded: minds != nil}
			s.begin(&w, ref(zs, i), ref(movers, i), ref(owned, i), ref(colls, i), ref(paces, i))
			if hands != nil && hands[i].Steers() {
				if w.o.Leg.Active {
					s.releaseLeg(w.o.Leg, id)
					s.occupancy.Enter(cells[i].Cell, w.onDeck, id, w.domain)
				}
				cells[i].Next = 0
				cb.RemoveCompOne(id, s.orderID)
				continue
			}
			if arrived != nil {
				cb.RemoveCompOne(id, s.arrivedID)
			}
			switch s.head(&w, changed, d) {
			case asking:
				s.asked, s.askedIn = append(s.asked, id), append(s.askedIn, w.entered)
			case onward:
				s.tail(&w, d)
			}
			if w.arrived {
				arrivedIDs = append(arrivedIDs, id)
			}
		}

		if len(arrivedIDs) > 0 {
			buf := s.query.BeginMigrate(cb)
			for _, id := range arrivedIDs {
				buf.Add(id)
			}
			buf.Commit(s.arrivedEditor)
		}
		s.finished = arrivedIDs
		s.supply(cb)
	}
	if len(s.asked) > 0 {
		s.walkOn(cb, d)
	}
	s.touch(cb, d)
	s.tell(cb, d)
	for _, a := range s.arrivals {
		cb.AddOne(a.id, s.arrivedID, order.Arrived{Cell: a.cell})
	}
	s.arrivals = s.arrivals[:0]
	clear(s.told)
	s.wanted, s.wanting = s.wanting, s.wanted
	clear(s.wanting)
}

// a unit with an order, as an Update moves it
type walker struct {
	id              uid.UID64
	o               *order.MoveOrder
	at              *unit.At
	base            *world.Base
	st              steering.Helm
	state           *tag.Tags[unit.States] // nil: its chunk has none, the id goes to lacking
	last            *order.LastOrder       // nil: its chunk has none, the id goes to lastIDs
	coll            *collision.Collider    // nil: it touches nobody
	minded          bool
	domain          cell.Domain
	current, actual cell.ID
	onDeck          bool
	m               bodies.Member
	entered         bool
	arrived         bool
}

func ref[T any](s []T, i int) *T {
	if s == nil {
		return nil
	}
	return &s[i]
}

func (s *System) begin(w *walker, z *world.Z, mover *unit.Mover, owners *tag.Tags[owner.Family],
	coll *collision.Collider, pace *steering.Pace,
) {
	w.domain, w.current = unit.DomainAt(nil, 0), w.at.Cell
	if mover != nil {
		w.domain = mover.Domain
	}
	var ok bool
	if w.actual, w.onDeck, ok = spacing.LevelOf(s.grid, s.terrain, w.base.Pos.Center(), z); !ok {
		w.actual = w.current
	}
	w.m = bodies.Member{ID: w.id, Cell: w.current, From: w.current, Leg: w.o.Leg, Domain: w.domain,
		Pos: w.base.Pos, Vel: w.base.Vel.Delta(), Facing: w.base.Vel.Dir, Order: w.o,
		Brake: w.st.Braking(), Share: 1, MaxSpeed: w.st.MaxSpeed}
	if pace != nil {
		w.m.Share = pace.Share
	}
	if z != nil {
		w.m.Z = *z
	}
	if mover != nil {
		w.m.Lift = mover.Lift
	}
	if by, ok := s.wanted[w.id]; ok {
		w.m.PressedBy, w.m.Pressed = by, true
	}
	if owners != nil {
		w.m.Owners = *owners
	}
	if w.coll = coll; coll != nil {
		w.m.Touching = waysOff(coll.Contacts())
	}
}

// two boxes part, or rest, along one axis, so each contact's way off lies along it
func waysOff(contacts []collision.Contact) geom.Vec {
	var lo, hi [2]bool
	for _, c := range contacts {
		if c.Terrain || c.Sensed || c.Other == 0 {
			continue
		}
		lo[0], hi[0] = lo[0] || c.Normal.X < 0, hi[0] || c.Normal.X > 0
		lo[1], hi[1] = lo[1] || c.Normal.Y < 0, hi[1] || c.Normal.Y > 0
	}
	return geom.NewVec(side(lo[0], hi[0]), side(lo[1], hi[1]))
}

func side(lo, hi bool) float32 {
	switch {
	case lo && !hi:
		return -1
	case hi && !lo:
		return 1
	}
	return 0
}

type stepped uint8

const (
	stood stepped = iota
	asking
	onward
)

// head brings w's order up to where w stands and what it was told, up to its route
func (s *System) head(w *walker, changed bool, d time.Duration) stepped {
	o, st := w.o, w.st
	p, leg := &o.Path, &o.Leg
	w.at.Next = 0 // led nowhere unless its route leads it below
	if changed && p.Index < p.Length && !s.admitsAll(p.Steps[p.Index:p.Length], w.domain) {
		p.Length = 0
	}
	if !leg.Active && p.Index < p.Length && w.actual != order.At(p) {
		p.Length = 0 // moved off its route by another, the board following it
	}

	o.Cooldown = max(o.Cooldown-d, 0)
	o.Retry = max(o.Retry-d, 0)
	o.TrailFor = max(o.TrailFor-d, 0)
	if o.TrailFor > 0 && s.against(w, o.Trailing) {
		o.TrailFor = spacing.TrailMemory
	}
	if o.Bumped {
		o.Bumped = false
		answer := s.keep.Bump(w.m, o)
		if answer == spacing.GiveUp {
			st.RequestSpeed(0)
			s.arrive(w)
			return stood
		}
		if answer == spacing.StopAndPlan {
			o.Cooldown = bumpInterval
			if leg.Active {
				s.releaseLeg(*leg, w.id)
				s.occupancy.Enter(w.actual, w.onDeck, w.id, w.domain)
			} else if w.actual != w.current {
				s.occupancy.Leave(w.current, w.id)
				s.occupancy.Enter(w.actual, w.onDeck, w.id, w.domain)
			}
			s.moveTo(w, w.actual)
			*leg = order.Leg{}
			p.Length = 0
			st.RequestSpeed(0)
			return stood
		}
	}

	switch {
	case leg.Active && w.actual == leg.From && !s.admitsLeg(*leg, w.domain):
		s.releaseLeg(*leg, w.id)
		s.occupancy.Enter(w.actual, w.onDeck, w.id, w.domain)
		*leg = order.Leg{}
		p.Length = 0
		st.RequestSpeed(0)
	case leg.Active && !order.Covers(*leg, w.actual):
		s.releaseLeg(*leg, w.id)
		s.occupancy.Enter(w.actual, w.onDeck, w.id, w.domain)
		s.moveTo(w, w.actual)
		*leg = order.Leg{}
		p.Length = 0
	case leg.Active && (w.actual == leg.From || w.actual == leg.To):
		s.moveTo(w, w.actual)
	case !leg.Active && w.actual != w.current:
		s.occupancy.Leave(w.current, w.id)
		s.occupancy.Enter(w.actual, w.onDeck, w.id, w.domain)
		s.moveTo(w, w.actual)
		p.Length = 0
	}
	w.m.Cell, w.m.From, w.m.Leg = w.at.Cell, w.at.Cell, *leg

	if !leg.Active && !s.terrain.Kind(w.at.Cell).Admits(w.domain) {
		// frozen in, say: the order waits for the ground to change
		st.RequestSpeed(0)
		p.Length = 0
		return stood
	}

	stop, held := s.carryOut(w.m, o, st, d)
	if stop {
		st.RequestSpeed(0)
		if leg.Active {
			s.releaseLeg(*leg, w.id)
			s.occupancy.Enter(w.actual, w.onDeck, w.id, w.domain)
			s.moveTo(w, w.actual)
			*leg = order.Leg{}
		}
		s.arrive(w)
		return stood
	}
	if held {
		return stood
	}

	s.keep.Ready(w.m, o)
	if !leg.Active && (p.Length == 0 || p.Index >= p.Length) &&
		(w.at.Cell != o.Target || !s.keep.On(w.m, o.Target, o.Level)) {
		// waiting on a search that found nothing: the same answer till its Retry ends
		if o.Waited == 0 || o.Retry == 0 {
			o.Ask = order.AskRoute
			return asking
		}
		st.RequestSpeed(0)
		if o.Waited += d; o.Waited < spacing.TargetWaitTimeout {
			return stood
		}
		o.Ask = order.AskNearest
		return asking
	}
	return onward
}

// whether w touches whom or has them within a body's reach, who are under way and ahead of it
// still: a queue, packed or loose, trails on with no strike, and a trail it has turned round lapses
func (s *System) against(w *walker, whom uid.UID64) bool {
	b, ok := s.keep.Other(whom)
	self, known := s.keep.Other(w.id)
	if !ok || !known || !b.Alive() {
		return false
	}
	to := s.unwrap(self.At, b.At)
	d := geom.NewVec(to.X-self.At.X, to.Y-self.At.Y)
	if ahead, _, _ := s.bearing(self, b, geom.NewVec(-d.X, -d.Y)); !ahead {
		return false
	}
	return touches(w.coll, whom) || withinReach(self, b, d)
}

func touches(coll *collision.Collider, whom uid.UID64) bool {
	if coll == nil {
		return false
	}
	for _, c := range coll.Contacts() {
		if !c.Terrain && !c.Sensed && c.Other == whom {
			return true
		}
	}
	return false
}

// whether other's box lies no more than a body's width past self's along the way self's route
// leads it, or its facing with none, d leading from self to other
func withinReach(self, other bodies.Body, d geom.Vec) bool {
	u := self.Way
	if u == (geom.Vec{}) {
		u = self.Facing
	}
	gap := d.X*u.X + d.Y*u.Y - extent(self.Half, u) - extent(other.Half, u)
	// a body's width, the gap the spacing keeps beside a body (spacing's gapFor)
	return gap <= 2*max(self.Half.X, self.Half.Y)*spacing.SpacingGap
}

// the half of a box of half sizes h along the unit vector u
func extent(h, u geom.Vec) float32 { return h.X*f32.Abs(u.X) + h.Y*f32.Abs(u.Y) }

// walkOn has the units that asked the Routing go on with its answers, the same step
func (s *System) walkOn(cb *goke.CmdBuf, d time.Duration) {
	if s.Routing != nil {
		s.Routing.Update(cb, d)
	}
	for k, id := range s.asked {
		if !s.outside.Seek(id) {
			continue
		}
		cur := s.outside.Cursor()
		w := walker{id: id, o: s.order.At(cur), at: s.cell.At(cur), base: s.base.At(cur),
			st:    steering.Helm{Steering: s.steer.At(cur), Course: s.course.At(cur)},
			state: s.states.At(cur), last: s.lastOrder.At(cur), minded: s.mind.Present(cur),
			entered: s.askedIn[k]}
		s.begin(&w, s.z.At(cur), s.mover.At(cur), s.owners.At(cur), s.coll.At(cur),
			s.pace.At(cur))
		ask := w.o.Ask
		w.o.Ask = order.AskNone
		switch ask {
		case order.AskNone:
			s.tail(&w, d)
		case order.NoPlace:
			w.st.RequestSpeed(0)
			s.arrive(&w)
		default:
			w.st.RequestSpeed(0)
		}
		if w.arrived {
			cb.RemoveCompOne(id, s.orderID)
		}
	}
	s.supply(cb)
}

// tail drives w along its route
func (s *System) tail(w *walker, d time.Duration) {
	o, st := w.o, w.st
	p, leg := &o.Path, &o.Leg
	target := o.Target
	dt := float32(d.Seconds())

	if leg.Active && p.Index < p.Length && p.Steps[p.Index] == leg.From {
		// the route goes back the way the leg came: the same cells held
		leg.From, leg.To = leg.To, leg.From
		leg.FromDeck, leg.ToDeck = leg.ToDeck, leg.FromDeck
	}

	waypoint, aloft := target, false
	switch {
	case leg.Active:
		waypoint = leg.To
	case p.Length > 0 && p.Index < p.Length:
		waypoint, aloft = p.Steps[p.Index], order.Deck(p, p.Index)
	}

	if !leg.Active && waypoint != w.at.Cell {
		reserved, why := s.reserveLeg(order.Spot{C: w.at.Cell, Deck: w.onDeck},
			order.Spot{C: waypoint, Deck: aloft}, w.id, w.domain)
		if !reserved.Active {
			st.RequestSpeed(0)
			switch {
			case !why.held:
				p.Length = 0
			case why.corner:
				// a held corner of a slantwise step is gone round square, not through
				order.Learn(o, why.cell)
				p.Length = 0
			default:
				if holder, known := s.keep.Blocked(w.m, o, why.cell, why.deck, d); known {
					s.touched = append(s.touched, touching{self: w.id, other: holder,
						way: s.wayBetween(why.cell, w.at.Cell), cell: why.cell, from: w.at.Cell,
						headOn: w.m.Pressed && w.m.PressedBy == holder, refused: true})
					if s.wanting == nil {
						s.wanting = map[uid.UID64]uid.UID64{}
					}
					s.wanting[holder] = w.id
				}
			}
			return
		}
		*leg = reserved
		o.Holding, o.WaitedOut, o.Cornered = 0, false, false
	}
	w.at.Next = waypoint

	have := w.base.Pos.Center()
	end := s.goal(target, o.Spot)
	want := s.unwrap(have, s.grid.CellCenter(waypoint))
	// the goal's cell met on the way, its deck over the goal under it, is a waypoint
	final := waypoint == target && !order.Through(p, waypoint)
	if final {
		want = s.unwrap(have, end)
	}
	s.keep.Watch(w.m, o, waypoint, want, d)

	reach := lookaheadReach(st, dt)
	if final && o.Queued > 0 && o.Linger > 0 {
		dx, dy := want.X-have.X, want.Y-have.Y
		if dist, within := f32.Hypot(dx, dy), s.keep.Within(w.m); dist > within {
			dir := geom.NewVec(dx, dy)
			s.keep.Steer(w.m, st, dir, approach(st, dist, within)*turnFactor(w.base.Vel.Dir, dir))
			return
		}
		st.RequestSpeed(0)
		if leg.Active {
			s.releaseLeg(*leg, w.id)
			s.occupancy.Enter(leg.To, leg.ToDeck, w.id, w.domain)
			s.moveTo(w, leg.To)
			*leg = order.Leg{}
		}
		if o.Linger = max(o.Linger-d, 0); o.Linger > 0 {
			return
		}
	}
	if final && o.Queued > 0 {
		from := w.at.Cell
		if leg.Active {
			from = leg.From
		}
		if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
			s.drive(w.m, st, w.base.Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end),
				reach, st.MaxSpeed)
			return
		}
		if leg.Active && w.actual != leg.To {
			// passed by the lookahead, not yet entered: the leg held, the next goal aimed
			// at
			next := o.Waypoints[0]
			s.route = append(s.route[:0], s.unwrap(have, s.goal(next.Cell, next.Spot)))
			s.drive(w.m, st, w.base.Vel.Dir, have, s.route, reach, st.MaxSpeed)
			return
		}
		if leg.Active {
			s.releaseLeg(*leg, w.id)
			s.occupancy.Enter(leg.To, leg.ToDeck, w.id, w.domain)
			s.moveTo(w, leg.To)
			*leg = order.Leg{}
		}
		order.Advance(o)
		s.route = append(s.route[:0], s.unwrap(have, s.goal(o.Target, o.Spot)))
		s.drive(w.m, st, w.base.Vel.Dir, have, s.route, reach, st.MaxSpeed)
		return
	}

	if !final {
		from := w.at.Cell
		if leg.Active {
			from = leg.From
		}
		if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
			s.drive(w.m, st, w.base.Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end),
				reach, st.MaxSpeed)
			return
		}
		if leg.Active && w.actual != leg.To {
			// passed by the lookahead, not yet entered: a wide turner looks further ahead
			// than half a cell, and let go the next tick finds it short of its cell and
			// plans again
			s.drive(w.m, st, w.base.Vel.Dir, have,
				s.ahead(have, want, p, waypoint, target, end)[1:], reach, st.MaxSpeed)
			return
		}
		if leg.Active {
			s.releaseLeg(*leg, w.id)
			s.occupancy.Enter(leg.To, leg.ToDeck, w.id, w.domain)
			s.moveTo(w, leg.To)
			*leg = order.Leg{}
		}
		if p.Index < p.Length && p.Steps[p.Index] == waypoint {
			p.Index++
		}
		// the next waypoint is aimed at now, reserved next tick
		if p.Index < p.Length {
			w.at.Next = p.Steps[p.Index]
		}
		s.drive(w.m, st, w.base.Vel.Dir, have,
			s.ahead(have, want, p, waypoint, target, end)[1:], reach, st.MaxSpeed)
		return
	}

	dx, dy := want.X-have.X, want.Y-have.Y
	dist := f32.Hypot(dx, dy)
	if within := s.keep.Within(w.m); dist > within {
		dir := geom.NewVec(dx, dy)
		s.keep.Steer(w.m, st, dir, approach(st, dist, within)*turnFactor(w.base.Vel.Dir, dir))
		return
	}

	st.RequestSpeed(0)
	st.Speed = 0
	if face := o.Face; face != (geom.Vec{}) {
		st.Request(geom.NewVec(face.X-want.X, face.Y-want.Y))
	}

	if s.space != nil && (f32.Abs(dx) > f32.Eps(want.X) || f32.Abs(dy) > f32.Eps(want.Y)) {
		s.space.Move(&w.base.Pos.AABB, geom.NewVec(dx, dy))
	}

	if leg.Active {
		s.releaseLeg(*leg, w.id)
		s.occupancy.Enter(leg.To, leg.ToDeck, w.id, w.domain)
		s.moveTo(w, leg.To)
		*leg = order.Leg{}
	}
	if at, deck, ok := spacing.LevelOf(s.grid, s.terrain, want, &w.m.Z); ok && at != w.at.Cell {
		s.occupancy.Leave(w.at.Cell, w.id)
		s.occupancy.Enter(at, deck, w.id, w.domain)
		s.moveTo(w, at)
	}

	if p.Index < p.Length && p.Steps[p.Index] == waypoint {
		p.Index++
	}

	s.arrive(w)
}

func (s *System) arrive(w *walker) {
	w.arrived = true
	w.at.Next = 0
	switch {
	case w.o.Group == 0: // giving way, or looking: the group stays
	case w.last != nil:
		w.last.Group = w.o.Group
	default:
		s.lastIDs = append(s.lastIDs, w.id)
		s.lastLacking = append(s.lastLacking, order.LastOrder{Group: w.o.Group})
	}
	if w.minded {
		s.arrivals = append(s.arrivals, arrival{id: w.id, cell: w.at.Cell})
	}
}

func (s *System) moveTo(w *walker, c cell.ID) {
	if c == w.at.Cell {
		return
	}
	w.at.Cell, w.at.Next = c, 0
	w.o.Retry = 0 // from another cell, another search
	if w.entered {
		return
	}
	w.entered = true
	if w.state != nil {
		*w.state = w.state.With(unit.Entered)
	} else {
		s.lacking = append(s.lacking, w.id)
	}
}

// the States and LastOrders their chunks lack, after the chunks' own changes: they move the entity
func (s *System) supply(cb *goke.CmdBuf) {
	for _, id := range s.lacking {
		cb.AddOne(id, s.statesID, tag.Tags[unit.States](0).With(unit.Entered))
	}
	s.lacking = s.lacking[:0]
	for k, id := range s.lastIDs {
		cb.AddOne(id, s.lastOrderID, s.lastLacking[k])
	}
	s.lastIDs, s.lastLacking = s.lastIDs[:0], s.lastLacking[:0]
}

type arrival struct {
	id   uid.UID64
	cell cell.ID
}

// a unit vector, the short way round
func (s *System) wayBetween(a, b cell.ID) geom.Vec {
	from := s.grid.CellCenter(a)
	to := s.unwrap(from, s.grid.CellCenter(b))
	d := geom.NewVec(to.X-from.X, to.Y-from.Y)
	if l := f32.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

func (s *System) standing(cb *goke.CmdBuf, cursor *goke.Cursor) {
	if len(s.told) == 0 || s.hand.Slice(cursor) != nil {
		return
	}
	cells, bases, movers, zs := s.cell.Slice(cursor), s.base.Slice(cursor), s.mover.Slice(cursor), s.z.Slice(cursor)
	for i, id := range cursor.IDs {
		t, ok := s.told[id]
		if !ok || !t.aside {
			continue
		}
		of, ok := s.keep.Other(t.asideOf)
		if !ok {
			continue
		}
		m := bodies.Member{ID: id, Cell: cells[i].Cell, From: cells[i].Cell, Domain: unit.DomainAt(movers, i), Pos: bases[i].Pos, Vel: bases[i].Vel.Delta(), Facing: bases[i].Vel.Dir}
		if zs != nil {
			m.Z = zs[i]
		}
		if movers != nil {
			m.Lift = movers[i].Lift
		}
		if order, ok := s.keep.StepAside(m, of); ok {
			cb.AddOne(id, s.orderID, order)
		}
	}
}

type touching struct {
	self, other uid.UID64
	way         geom.Vec // self leaving other
	cell, from  cell.ID  // refused: the cell other holds, the one self was refused it from
	headOn      bool
	refused     bool
}

// each pair once: collision records a contact on the one that struck, or on both
func (s *System) feel() {
	clear(s.felt)
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		colls := s.coll.Slice(cursor)
		if colls == nil {
			continue
		}
		for i, id := range cursor.IDs {
			for _, c := range colls[i].Contacts() {
				// a unit learns by striking, not by leaning on a neighbour
				if c.Terrain || c.Other == 0 || c.Resting {
					continue
				}
				pair := [2]uid.UID64{min(id, c.Other), max(id, c.Other)}
				if !s.felt[pair] {
					s.felt[pair] = true
					s.touched = append(s.touched, touching{self: id, other: c.Other, way: c.Normal})
				}
			}
		}
	}
}

func (s *System) touch(cb *goke.CmdBuf, d time.Duration) {
	hosted := s.Touches != nil && !s.Touches.Empty()
	tick := s.Tick.Of(d)
	clear(s.felt)
	for _, t := range s.touched {
		if t.refused {
			s.felt[[2]uid.UID64{t.self, t.other}] = true
		}
	}
	for _, t := range s.touched {
		self, ok := s.keep.Other(t.self)
		other, found := s.keep.Other(t.other)
		if !ok || !found || self.Domain&other.Domain == 0 {
			continue
		}
		cell, from := other.Cell, self.Cell
		if t.refused {
			cell, from = t.cell, t.from
		}
		head := t.headOn || t.refused && s.felt[[2]uid.UID64{t.other, t.self}] || !t.refused && headOn(self, other, t.way)
		s.fire(tick, hosted, self, other, t.way, cell, head)
		s.fire(tick, hosted, other, self, geom.NewVec(-t.way.X, -t.way.Y), from, head)
	}
}

func (s *System) fire(tick host.Tick, hosted bool, self, other bodies.Body, way geom.Vec, cell cell.ID, head bool) {
	t := order.Touch{Self: self.ID, Other: other.ID, Way: way, Moving: self.Moving, OtherMoving: other.Moving,
		GivingWay: self.GivingWay, OtherGivingWay: other.GivingWay, LastGoal: self.LastGoal,
		Ally: owner.Allies(self.Owners, other.Owners), Groupmate: self.Group != 0 && self.Group == other.Group,
		HeadOn: head && self.Moving && other.Moving, WaitedOut: self.WaitedOut, Cornered: self.Cornered}
	t.Ahead, t.Along, t.SameWay = s.bearing(self, other, way)
	t.OtherTrailing = other.Trailing
	if self.Moving {
		t.OnMyGoal = s.keep.OnGoal(self, other)
		t.NearMyGoal = s.grid.Distance(other.Cell, self.Goal) <= nearGoal
		if !other.Moving {
			_, t.Room = s.keep.StepAside(other.Member(), self)
		}
		if self.Minded {
			if s.blocking == nil {
				s.blocking = map[uid.UID64]order.Blocked{}
			}
			s.blocking[self.ID] = blockedOf(t, cell)
		}
	}
	if hosted {
		s.Touches.Dispatch(tick, s.marksOf(self.ID), s.marksOf(other.ID), t)
	}
}

// cells from a unit's goal within which its group gathers
const nearGoal = 2

// the cosine of the eighth of a turn either side of a way, within which another lies ahead on it
// or goes it too
const quarterTurn = 0.7

// how other stands to self's way: ahead within a quarter turn of it, moving along it, its route
// leading that way too; with none, the same by self's facing, way leading self off other
func (s *System) bearing(self, other bodies.Body, way geom.Vec) (ahead, along, same bool) {
	w := self.Way
	if w == (geom.Vec{}) {
		f := self.Facing
		return way.X*f.X+way.Y*f.Y < 0, other.Vel.X*f.X+other.Vel.Y*f.Y > 0,
			other.Facing.X*f.X+other.Facing.Y*f.Y > 0
	}
	at := s.unwrap(self.At, other.At)
	d := geom.NewVec(at.X-self.At.X, at.Y-self.At.Y)
	ahead = d.X*w.X+d.Y*w.Y > quarterTurn*f32.Hypot(d.X, d.Y)
	along = other.Vel.X*w.X+other.Vel.Y*w.Y > 0
	same = other.Way.X*w.X+other.Way.Y*w.Y > quarterTurn
	return ahead, along, same
}

func (s *System) marksOf(id uid.UID64) host.Marks {
	if !s.marks.Seek(id) {
		return host.Marks{}
	}
	return s.Touches.Read(0, s.marksCols.Row(s.marks.Cursor()), int(s.marks.Cursor().Slot))
}

// way is the way a leaves b
func headOn(a, b bodies.Body, way geom.Vec) bool {
	towards := func(v, dir geom.Vec) bool {
		l := f32.Hypot(v.X, v.Y)
		return l > 1e-9 && (v.X*dir.X+v.Y*dir.Y)/l > 0.5
	}
	return a.Moving && b.Moving && towards(a.Vel, geom.NewVec(-way.X, -way.Y)) && towards(b.Vel, way)
}

func (s *System) tell(cb *goke.CmdBuf, d time.Duration) {
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		if s.mind.Slice(cursor) == nil {
			continue
		}
		facts, orders := s.blocked.Slice(cursor), s.order.Slice(cursor)
		for i, id := range cursor.IDs {
			if b, ok := s.blocking[id]; ok && orders != nil {
				cb.AddOne(id, s.blockedID, b)
				continue
			}
			if facts == nil {
				continue
			}
			f := &facts[i]
			if f.Lasts -= d; f.Lasts > 0 && orders != nil {
				continue
			}
			cb.RemoveCompOne(id, s.blockedID)
		}
	}
	clear(s.blocking)
}

func (s *System) carryOut(m bodies.Member, o *order.MoveOrder, st steering.Helm, d time.Duration) (stop, held bool) {
	if t, ok := s.told[m.ID]; ok {
		switch {
		case t.stop:
			return true, false
		case t.aside:
			s.stepAside(m, o, t.asideOf)
		case t.detour:
			if other, ok := s.keep.Other(t.detourOf); ok && s.keep.Detour(m, o, other) == spacing.GiveUp {
				return true, false
			}
		case t.pass:
			if other, ok := s.keep.Other(t.passOf); ok {
				s.keep.Pass(m, o, other)
			}
		case t.hold:
			if o.Holding <= 0 && !o.WaitedOut {
				o.Holding = spacing.StallAfter
			}
		case t.settle:
			if other, ok := s.keep.Other(t.settleBeside); ok {
				s.keep.Settle(m, o, other)
			}
		case t.trail:
			if other, ok := s.keep.Other(t.trailOf); ok {
				if o.TrailFor == 0 { // a renewal goes on with the wait, a new trail starts it
					o.Trailed = 0
				}
				o.Trailing, o.TrailFor = other.ID, spacing.TrailMemory
				o.TrailPace = other.MaxSpeed * other.Share
				if other.Trailing { // a queue goes at the pace of its head
					o.TrailPace = min(o.TrailPace, other.TrailPace)
				}
			}
		}
	}
	if o.Holding <= 0 {
		return false, false
	}
	next, aloft := o.Target, false
	if o.Path.Index < o.Path.Length {
		next, aloft = o.Path.Steps[o.Path.Index], order.Deck(&o.Path, o.Path.Index)
	}
	if s.keep.MayStep(m, next, aloft, s.wayTo(m, next), nil) {
		o.Holding = 0
		return false, false
	}
	st.RequestSpeed(0)
	if o.Holding -= d; o.Holding <= 0 {
		o.Holding, o.WaitedOut = 0, true
	}
	return false, true
}

func (s *System) stepAside(m bodies.Member, o *order.MoveOrder, of uid.UID64) {
	other, ok := s.keep.Other(of)
	if !ok {
		return
	}
	aside, ok := s.keep.StepAside(m, other)
	if !ok {
		return
	}
	aside.Linger, aside.Face, aside.Group = spacing.YieldLinger, o.Face, o.Group
	aside.Enqueue(order.Goal{Cell: o.Target, Spot: o.Spot, At: o.At})
	for _, g := range o.Waypoints[:o.Queued] {
		aside.Enqueue(g)
	}
	if o.Leg.Active {
		aside.Leg = o.Leg
	}
	*o = aside
}

// a unit vector
func (s *System) wayTo(m bodies.Member, c cell.ID) geom.Vec {
	have := m.Centre()
	to := s.unwrap(have, s.grid.CellCenter(c))
	d := geom.NewVec(to.X-have.X, to.Y-have.Y)
	if l := f32.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

func (s *System) gather(dst []bodies.Body) []bodies.Body {
	for s.query.Paced(0); s.query.Next(); {
		cursor := s.query.Cursor()
		bases, orders, movers := s.base.Slice(cursor), s.order.Slice(cursor), s.mover.Slice(cursor)
		cells, owned, lasts := s.cell.Slice(cursor), s.owners.Slice(cursor), s.lastOrder.Slice(cursor)
		zs, minded := s.z.Slice(cursor), s.mind.Slice(cursor) != nil
		steers, paces := s.steer.Slice(cursor), s.pace.Slice(cursor)
		for i, id := range cursor.IDs {
			pos := bases[i].Pos
			b := bodies.Body{ID: id, At: pos.Center(), Half: geom.NewVec(pos.Size.X/2, pos.Size.Y/2), Vel: bases[i].Vel.Delta(), Domain: unit.DomainAt(movers, i),
				Moving: orders != nil, Cell: cells[i].Cell, Minded: minded, Facing: bases[i].Vel.Dir}
			b.Share, b.MaxSpeed = 1, steers[i].MaxSpeed
			if paces != nil {
				b.Share = paces[i].Share
			}
			if zs != nil {
				b.Z = zs[i]
			}
			if movers != nil {
				b.Lift = movers[i].Lift
			}
			if lasts != nil {
				b.Group = lasts[i].Group
			}
			if orders != nil {
				o := &orders[i]
				b.GivingWay, b.Goal, b.Spot, b.LastGoal = o.GivingWay, o.Target, s.goal(o.Target, o.Spot), o.Queued == 0
				b.WaitedOut, b.Cornered = o.WaitedOut, o.Cornered
				b.Trailing, b.TrailPace, b.Way = o.TrailFor > 0, o.TrailPace, s.wayOf(b.At, o)
				if o.Group != 0 {
					b.Group = o.Group
				}
			}
			if owned != nil {
				b.Owners = owned[i]
			}
			dst = append(dst, b)
		}
	}
	return dst
}

// the way o leads one at at next, a unit vector: to its step's end, else its route's next cell,
// else its target; zero there
func (s *System) wayOf(at geom.Vec, o *order.MoveOrder) geom.Vec {
	next := o.Target
	switch {
	case o.Leg.Active:
		next = o.Leg.To
	case o.Path.Index < o.Path.Length:
		next = o.Path.Steps[o.Path.Index]
	}
	to := s.unwrap(at, s.grid.CellCenter(next))
	d := geom.NewVec(to.X-at.X, to.Y-at.Y)
	if l := f32.Hypot(d.X, d.Y); l > 0 {
		return geom.NewVec(d.X/l, d.Y/l)
	}
	return geom.Vec{}
}

func (s *System) goal(c cell.ID, spot geom.Vec) geom.Vec {
	if spot == (geom.Vec{}) {
		return s.grid.CellCenter(c)
	}
	return spot
}

// the short way round on a wrapping axis
func (s *System) unwrap(have, to geom.Vec) geom.Vec {
	if s.space == nil {
		return to
	}
	return geom.NewVec(
		have.X+spacing.ShortestAxisDelta(have.X, to.X, s.space.Width, s.space.Edges.WrapsX()),
		have.Y+spacing.ShortestAxisDelta(have.Y, to.Y, s.space.Height, s.space.Edges.WrapsY()),
	)
}

func (s *System) ahead(have, want geom.Vec, p *order.Path, waypoint, target cell.ID, end geom.Vec) []geom.Vec {
	s.route = append(s.route[:0], want)
	if waypoint == target && !order.Through(p, waypoint) {
		return s.route
	}
	k := int(p.Index)
	if k < int(p.Length) && p.Steps[k] == waypoint {
		k++
	}
	last := want
	for ; k < int(p.Length) && len(s.route) < maxAhead; k++ {
		c := s.grid.CellCenter(p.Steps[k])
		if p.Steps[k] == target && k == int(p.Length)-1 {
			c = end
		}
		last = s.unwrap(last, c)
		s.route = append(s.route, last)
	}
	return s.route
}

const maxAhead = 8

func (s *System) drive(m bodies.Member, st steering.Helm, heading, have geom.Vec, route []geom.Vec, reach, speed float32,
) {
	at := lookahead(have, route, reach)
	if at == have {
		s.keep.Steer(m, st, geom.Vec{}, speed)
		return
	}
	dir := geom.NewVec(at.X-have.X, at.Y-have.Y)
	s.keep.Steer(m, st, dir, speed*turnFactor(heading, dir))
}

// the turning radius at the current speed
func lookaheadReach(st steering.Helm, dt float32) float32 {
	if st.TurnRate <= 0 {
		return 0
	}
	return f32.Abs(st.Speed) * dt / st.TurnRate
}

// TurnCrawl is the share of speed kept through the sharpest turn.
const TurnCrawl = 0.2

func turnFactor(heading, dir geom.Vec) float32 {
	h, d := f32.Hypot(heading.X, heading.Y), f32.Hypot(dir.X, dir.Y)
	if h == 0 || d == 0 {
		return 1
	}
	return min(max((heading.X*dir.X+heading.Y*dir.Y)/(h*d), TurnCrawl), 1)
}

// the point reach along the polyline have → route[0] → route[1] …, or its end
func lookahead(have geom.Vec, route []geom.Vec, reach float32) geom.Vec {
	if len(route) == 0 {
		return have
	}
	if reach <= 0 {
		return route[0]
	}
	at := have
	for _, p := range route {
		dx, dy := p.X-at.X, p.Y-at.Y
		d := f32.Hypot(dx, dy)
		if d >= reach {
			if d == 0 {
				return p
			}
			return geom.NewVec(at.X+dx/d*reach, at.Y+dy/d*reach)
		}
		reach -= d
		at = p
	}
	return at
}

// beyond the plane through w perpendicular to from → w, or within reach of w
func passed(have, w, from geom.Vec, reach float32) bool {
	if f32.Hypot(have.X-w.X, have.Y-w.Y) <= max(reach, spacing.ArrivalEpsilon) {
		return true
	}
	ax, ay := w.X-from.X, w.Y-from.Y
	if ax == 0 && ay == 0 {
		return false
	}
	return (have.X-w.X)*ax+(have.Y-w.Y)*ay >= 0
}

// never below the speed braking would leave it at the arrival radius within
func approach(st steering.Helm, dist, within float32) float32 {
	brake := st.Braking()
	if brake <= 0 {
		return st.MaxSpeed
	}
	v := max(f32.Sqrt(2*brake*dist), f32.Sqrt(2*brake*within))
	return min(v, st.MaxSpeed)
}

type refusal struct {
	cell               cell.ID
	deck, held, corner bool
}

// every cell of the step is claimed, or none: an inactive Leg and why
func (s *System) reserveLeg(from, to order.Spot, id uid.UID64,
	domain cell.Domain) (order.Leg, refusal) {
	leg := order.Leg{From: from.C, To: to.C, FromDeck: from.Deck, ToDeck: to.Deck, Active: true}
	if c1, c2, diag := s.grid.DiagonalNeighbors(from.C, to.C); diag {
		leg.C1, leg.C2, leg.Diagonal, leg.Decked = c1, c2, true, s.decked(from.C, to.C)
	}
	held, n := order.Held(leg)
	for _, h := range held[1:n] {
		if !s.terrain.Kind(h.C).Admits(domain) {
			return order.Leg{}, refusal{cell: h.C, deck: h.Deck}
		}
		if !s.occupancy.CanEnter(h.C, h.Deck, id, domain) {
			return order.Leg{}, refusal{cell: h.C, deck: h.Deck, held: true, corner: h.C != to.C}
		}
	}
	for _, h := range held[:n] {
		s.occupancy.Enter(h.C, h.Deck, id, domain)
	}
	return leg, refusal{}
}

func (s *System) decked(from, to cell.ID) bool {
	r, ok := s.terrain.(interface{ Reaches(c, n cell.ID) bool })
	return ok && (r.Reaches(from, to) || r.Reaches(to, from))
}

func (s *System) admitsAll(cells []cell.ID, domain cell.Domain) bool {
	for _, c := range cells {
		if !s.terrain.Kind(c).Admits(domain) {
			return false
		}
	}
	return true
}

func (s *System) admitsLeg(leg order.Leg, domain cell.Domain) bool {
	held, n := order.Held(leg)
	for _, h := range held[1:n] {
		if !s.terrain.Kind(h.C).Admits(domain) {
			return false
		}
	}
	return true
}

func (s *System) releaseLeg(leg order.Leg, id uid.UID64) {
	held, n := order.Held(leg)
	for _, h := range held[:n] {
		s.occupancy.Leave(h.C, id)
	}
}

func (s *System) ordered(id uid.UID64) bool {
	return s.outside.Seek(id) && s.order.At(s.outside.Cursor()) != nil
}

// after a Populate as after a Load; the cells the units stand on the board holds itself
func (s *System) seedLegs() {
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		orders, movers := s.order.Slice(cursor), s.mover.Slice(cursor)
		if orders == nil {
			continue
		}
		for i, id := range cursor.IDs {
			if orders[i].Leg.Active {
				held, n := order.Held(orders[i].Leg)
				for _, h := range held[:n] {
					s.occupancy.Enter(h.C, h.Deck, id, unit.DomainAt(movers, i))
				}
			}
		}
	}
}
