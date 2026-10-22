package navigation

import (
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/store"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type navigationSystem struct {
	pace       goke.Opt
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
	order   goke.OptComp[MoveOrder]
	mover   goke.OptComp[unit.Mover]
	z       goke.OptComp[world.Z]
	coll    goke.OptComp[collision.Collider]
	hand    goke.OptComp[steering.Driven]
	route   []geom.Vec

	owners      goke.OptComp[tag.Tags[owner.Family]]
	lastOrder   goke.OptComp[LastOrder]
	lastOrderID goke.CompID
	lastLacking []LastOrder
	lastIDs     []uid.UID64
	mind        goke.OptComp[store.Mind]
	blocked     goke.OptComp[Blocked]
	arrived     goke.OptComp[Arrived]
	blockedID   goke.CompID
	arrivedID   goke.CompID
	blocking    map[uid.UID64]Blocked
	arrivals    []arrival

	given     *givenQueues
	told      map[uid.UID64]told
	touched   []touching
	felt      map[[2]uid.UID64]bool // the pairs touching this tick, each once; refused, one way
	touches   *host.PairRules[Touch]
	marksCols *cgoke.Columns
	bumps     bumps
	marks     *goke.Query
	markCell  goke.Comp[unit.At]
	tick      host.TickSource

	orderID goke.CompID

	arrivedEditor *goke.Editor

	states   goke.OptComp[tag.Tags[unit.States]]
	statesID goke.CompID
	lacking  []uid.UID64 // the ids whose chunk had no States family, to get it

	terrainSeen uint64
}

var _ goke.System = (*navigationSystem)(nil)

const bumpInterval = 500 * time.Millisecond

func newNavigationSystem(pathFinder *spacing.Finder, grid grid.Grid, terrain cell.Terrain, occupancy cell.Occupancy) *navigationSystem {
	s := &navigationSystem{
		grid:       grid,
		terrain:    terrain,
		occupancy:  occupancy,
		pathFinder: pathFinder,
		keep:       spacing.NewCellKeeping(pathFinder, occupancy),
	}
	s.gathering = s.gather
	return s
}

func (s *navigationSystem) withKeeping(k spacing.Keeping) *navigationSystem {
	s.keep, s.occupancy = k, k.Occupancy()
	return s
}

func (s *navigationSystem) BindSpace(space *aabbworld.Space) { s.space = space }

func (s *navigationSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.cell, &s.base, &s.steer, &s.course).
		Optional(&s.order).
		Optional(&s.mover).
		Optional(&s.z).
		Optional(&s.coll).
		Optional(&s.hand).
		Optional(&s.owners, &s.lastOrder).
		Optional(&s.mind, &s.blocked, &s.arrived).
		Optional(&s.states)
	if s.pace != nil {
		qb.With(s.pace)
	}
	s.query = qb.Build()
	s.outside = s.query.Fork()
	s.orderID = si.RegComp[MoveOrder]()
	s.blockedID, s.arrivedID, s.lastOrderID = si.RegComp[Blocked](), si.RegComp[Arrived](), si.RegComp[LastOrder]()
	s.arrivedEditor = s.query.NewEditorBuilder().Remove(goke.Remove[MoveOrder]()).Build()
	s.statesID = si.RegComp[tag.Tags[unit.States]]()
	s.seedLegs()
	if s.touches != nil {
		marks := si.NewQueryBuilder(&s.markCell)
		s.marksCols = cgoke.NewColumns(marks)
		s.touches.Bind(s.marksCols)
		s.marks = marks.Build()
	}
}

func (s *navigationSystem) Update(cb *goke.CmdBuf, d time.Duration) {
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
	if s.given != nil {
		if s.told == nil {
			s.told = map[uid.UID64]told{}
		}
		s.given.drain(s.told)
	}

	for s.query.Paced(d); s.query.Next(); {
		cursor := s.query.Cursor()
		orders := s.order.Slice(cursor)
		if orders == nil {
			s.standing(cb, cursor)
			continue
		}

		cells := s.cell.Slice(cursor)
		bases := s.base.Slice(cursor)
		states := s.states.Slice(cursor)

		arrivedIDs := s.finished[:0]

		steers, courses := s.steer.Slice(cursor), s.course.Slice(cursor)
		movers := s.mover.Slice(cursor)
		zs := s.z.Slice(cursor)
		minds, owned, arrived, lasts := s.mind.Slice(cursor), s.owners.Slice(cursor), s.arrived.Slice(cursor), s.lastOrder.Slice(cursor)
		hands := s.hand.Slice(cursor)
		dt := d.Seconds()

		for i, id := range cursor.IDs {
			domain := unit.DomainAt(movers, i)
			o := &orders[i]
			p := &o.Path
			leg := &o.Leg
			current := cells[i].Cell
			var z *world.Z
			if zs != nil {
				z = &zs[i]
			}
			actual, onDeck, ok := spacing.LevelOf(s.grid, s.terrain, bases[i].Pos.Center(), z)
			if !ok {
				actual = current
			}
			if hands != nil && hands[i].Steers() {
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(cells[i].Cell, onDeck, id, domain)
				}
				cb.RemoveCompOne(id, s.orderID)
				continue
			}
			st := steering.Helm{Steering: &steers[i], Course: &courses[i]}
			m := bodies.Member{ID: id, Cell: current, From: current, Leg: *leg, Domain: domain, Pos: bases[i].Pos, Vel: bases[i].Vel.Delta(), Facing: bases[i].Vel.Dir, Order: o}
			if zs != nil {
				m.Z = zs[i]
			}
			if movers != nil {
				m.Lift = movers[i].Lift
			}
			m.Brake = st.Braking()
			if by, ok := s.wanted[id]; ok {
				m.PressedBy, m.Pressed = by, true
			}
			if owned != nil {
				m.Owners = owned[i]
			}
			if arrived != nil {
				cb.RemoveCompOne(id, s.arrivedID)
			}
			arrive := func() {
				arrivedIDs = append(arrivedIDs, id)
				switch {
				case o.Group == 0: // giving way, or looking: the group stays
				case lasts != nil:
					lasts[i].Group = o.Group
				default:
					s.lastIDs, s.lastLacking = append(s.lastIDs, id), append(s.lastLacking, LastOrder{Group: o.Group})
				}
				if minds != nil {
					s.arrivals = append(s.arrivals, arrival{id: id, cell: cells[i].Cell})
				}
			}

			entered := false
			moveTo := func(c cell.ID) {
				if c == cells[i].Cell {
					return
				}
				cells[i].Cell = c
				o.Retry = 0 // from another cell, another search
				if !entered {
					entered = true
					s.enter(states, i, id)
				}
			}

			if changed && p.Index < p.Length && !s.admitsAll(p.Steps[p.Index:p.Length], domain) {
				p.Length = 0
			}
			if !leg.Active && p.Index < p.Length && actual != order.At(p) {
				p.Length = 0 // moved off its route by another, the board following it
			}

			o.Cooldown = max(o.Cooldown-d, 0)
			o.Retry = max(o.Retry-d, 0)
			if o.Bumped {
				o.Bumped = false
				answer := s.keep.Bump(m, o)
				if answer == spacing.GiveUp {
					st.RequestSpeed(0)
					arrive()
					continue
				}
				if answer == spacing.StopAndPlan {
					o.Cooldown = bumpInterval
					if leg.Active {
						s.releaseLeg(*leg, id)
						s.occupancy.Enter(actual, onDeck, id, domain)
					} else if actual != current {
						s.occupancy.Leave(current, id)
						s.occupancy.Enter(actual, onDeck, id, domain)
					}
					moveTo(actual)
					*leg = Leg{}
					p.Length = 0
					st.RequestSpeed(0)
					continue
				}
			}

			switch {
			case leg.Active && actual == leg.From && !s.admitsLeg(*leg, domain):
				s.releaseLeg(*leg, id)
				s.occupancy.Enter(actual, onDeck, id, domain)
				*leg = Leg{}
				p.Length = 0
				st.RequestSpeed(0)
			case leg.Active && !order.Covers(*leg, actual):
				s.releaseLeg(*leg, id)
				s.occupancy.Enter(actual, onDeck, id, domain)
				moveTo(actual)
				*leg = Leg{}
				p.Length = 0
			case leg.Active && (actual == leg.From || actual == leg.To):
				moveTo(actual)
			case !leg.Active && actual != current:
				s.occupancy.Leave(current, id)
				s.occupancy.Enter(actual, onDeck, id, domain)
				moveTo(actual)
				p.Length = 0
			}
			m.Cell, m.From, m.Leg = cells[i].Cell, cells[i].Cell, *leg

			if !leg.Active && !s.terrain.Kind(cells[i].Cell).Admits(domain) {
				// frozen in, say: the order waits for the ground to change
				st.RequestSpeed(0)
				p.Length = 0
				continue
			}

			stop, held := s.carryOut(m, o, st, d)
			if stop {
				st.RequestSpeed(0)
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(actual, onDeck, id, domain)
					moveTo(actual)
					*leg = Leg{}
				}
				arrive()
				continue
			}
			if held {
				continue
			}

			s.keep.Ready(m, o)
			target := o.Target
			if !leg.Active && (p.Length == 0 || p.Index >= p.Length) &&
				(cells[i].Cell != target || !s.keep.On(m, target, o.Level)) {
				var newPath Path
				found := false
				// waiting on a search that found nothing: the same answer till its Retry ends
				if o.Waited == 0 || o.Retry == 0 {
					newPath, found = s.keep.Route(m, cells[i].Cell, o)
				}
				if !found {
					st.RequestSpeed(0)
					o.Waited += d
					dest, destPath, wait, ok := s.keep.Lost(m, cells[i].Cell, o, o.Waited)
					if wait {
						continue
					}
					if !ok {
						arrive()
						continue
					}
					o.Target, target = dest, dest
					newPath = destPath
					order.Found(o)
				}
				o.Waited = 0
				*p = newPath
			}

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

			if !leg.Active && waypoint != cells[i].Cell {
				reserved, why := s.reserveLeg(order.Spot{C: cells[i].Cell, Deck: onDeck}, order.Spot{C: waypoint, Deck: aloft},
					id, domain)
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
						if holder, known := s.keep.Blocked(m, o, why.cell, why.deck, d); known {
							s.touched = append(s.touched, touching{self: id, other: holder, way: s.wayBetween(why.cell, cells[i].Cell),
								cell: why.cell, from: cells[i].Cell, headOn: m.Pressed && m.PressedBy == holder, refused: true})
							if s.wanting == nil {
								s.wanting = map[uid.UID64]uid.UID64{}
							}
							s.wanting[holder] = id
						}
					}
					continue
				}
				*leg = reserved
				o.Holding, o.WaitedOut, o.Cornered = 0, false, false
			}

			have := bases[i].Pos.Center()
			end := s.goal(target, o.Spot)
			want := s.unwrap(have, s.grid.CellCenter(waypoint))
			// the goal's cell met on the way, its deck over the goal under it, is a waypoint
			final := waypoint == target && !order.Through(p, waypoint)
			if final {
				want = s.unwrap(have, end)
			}
			s.keep.Watch(m, o, waypoint, want, d)

			reach := lookaheadReach(st, dt)
			if final && o.Queued > 0 && o.Linger > 0 {
				dx, dy := want.X-have.X, want.Y-have.Y
				if dist, within := math.Hypot(dx, dy), s.keep.Within(m); dist > within {
					dir := geom.NewVec(dx, dy)
					s.keep.Steer(m, st, dir, approach(st, dist, within)*turnFactor(bases[i].Vel.Dir, dir))
					continue
				}
				st.RequestSpeed(0)
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(leg.To, leg.ToDeck, id, domain)
					moveTo(leg.To)
					*leg = Leg{}
				}
				if o.Linger = max(o.Linger-d, 0); o.Linger > 0 {
					continue
				}
			}
			if final && o.Queued > 0 {
				from := cells[i].Cell
				if leg.Active {
					from = leg.From
				}
				if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
					s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end), reach, st.MaxSpeed)
					continue
				}
				if leg.Active && actual != leg.To {
					// passed by the lookahead, not yet entered: the leg held, the next goal aimed
					// at
					next := o.Waypoints[0]
					s.route = append(s.route[:0], s.unwrap(have, s.goal(next.Cell, next.Spot)))
					s.drive(m, st, bases[i].Vel.Dir, have, s.route, reach, st.MaxSpeed)
					continue
				}
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(leg.To, leg.ToDeck, id, domain)
					moveTo(leg.To)
					*leg = Leg{}
				}
				order.Advance(o)
				s.route = append(s.route[:0], s.unwrap(have, s.goal(o.Target, o.Spot)))
				s.drive(m, st, bases[i].Vel.Dir, have, s.route, reach, st.MaxSpeed)
				continue
			}

			if !final {
				from := cells[i].Cell
				if leg.Active {
					from = leg.From
				}
				if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
					s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end), reach, st.MaxSpeed)
					continue
				}
				if leg.Active && actual != leg.To {
					// passed by the lookahead, not yet entered: a wide turner looks further ahead
					// than half a cell, and let go the next tick finds it short of its cell and
					// plans again
					s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end)[1:], reach, st.MaxSpeed)
					continue
				}
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(leg.To, leg.ToDeck, id, domain)
					moveTo(leg.To)
					*leg = Leg{}
				}
				if p.Index < p.Length && p.Steps[p.Index] == waypoint {
					p.Index++
				}
				// the next waypoint is aimed at now, reserved next tick
				s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end)[1:], reach, st.MaxSpeed)
				continue
			}

			dx, dy := want.X-have.X, want.Y-have.Y
			dist := math.Hypot(dx, dy)
			if within := s.keep.Within(m); dist > within {
				dir := geom.NewVec(dx, dy)
				s.keep.Steer(m, st, dir, approach(st, dist, within)*turnFactor(bases[i].Vel.Dir, dir))
				continue
			}

			st.RequestSpeed(0)
			st.Speed = 0
			if face := o.Face; face != (geom.Vec{}) {
				st.Request(geom.NewVec(face.X-want.X, face.Y-want.Y))
			}

			if s.space != nil && (dx != 0 || dy != 0) {
				s.space.Move(&bases[i].Pos.AABB, geom.NewVec(dx, dy))
			}

			if leg.Active {
				s.releaseLeg(*leg, id)
				s.occupancy.Enter(leg.To, leg.ToDeck, id, domain)
				moveTo(leg.To)
				*leg = Leg{}
			}
			if at, deck, ok := spacing.LevelOf(s.grid, s.terrain, want, &m.Z); ok && at != cells[i].Cell {
				s.occupancy.Leave(cells[i].Cell, id)
				s.occupancy.Enter(at, deck, id, domain)
				moveTo(at)
			}

			if p.Index < p.Length && p.Steps[p.Index] == waypoint {
				p.Index++
			}

			arrive()
		}

		if len(arrivedIDs) > 0 {
			buf := s.query.BeginMigrate(cb)
			for _, id := range arrivedIDs {
				buf.Add(id)
			}
			buf.Commit(s.arrivedEditor)
		}
		s.finished = arrivedIDs
		for _, id := range s.lacking { // after the chunk's own changes: it moves the entity
			cb.AddOne(id, s.statesID, tag.Tags[unit.States](0).With(unit.Entered))
		}
		s.lacking = s.lacking[:0]
		for k, id := range s.lastIDs {
			cb.AddOne(id, s.lastOrderID, s.lastLacking[k])
		}
		s.lastIDs, s.lastLacking = s.lastIDs[:0], s.lastLacking[:0]
	}
	s.touch(cb, d)
	s.tell(cb, d)
	for _, a := range s.arrivals {
		cb.AddOne(a.id, s.arrivedID, Arrived{Cell: a.cell})
	}
	s.arrivals = s.arrivals[:0]
	clear(s.told)
	s.wanted, s.wanting = s.wanting, s.wanted
	clear(s.wanting)
}

type arrival struct {
	id   uid.UID64
	cell cell.ID
}

// a unit vector, the short way round
func (s *navigationSystem) wayBetween(a, b cell.ID) geom.Vec {
	from := s.grid.CellCenter(a)
	to := s.unwrap(from, s.grid.CellCenter(b))
	d := geom.NewVec(to.X-from.X, to.Y-from.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

func (s *navigationSystem) standing(cb *goke.CmdBuf, cursor *goke.Cursor) {
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
func (s *navigationSystem) feel() {
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

func (s *navigationSystem) touch(cb *goke.CmdBuf, d time.Duration) {
	hosted := s.touches != nil && !s.touches.Empty()
	tick := s.tick.Of(d)
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

func (s *navigationSystem) fire(tick host.Tick, hosted bool, self, other bodies.Body, way geom.Vec, cell cell.ID, head bool) {
	t := Touch{Self: self.ID, Other: other.ID, Way: way, Moving: self.Moving, OtherMoving: other.Moving,
		GivingWay: self.GivingWay, OtherGivingWay: other.GivingWay, LastGoal: self.LastGoal,
		Ally: owner.Allies(self.Owners, other.Owners), Groupmate: self.Group != 0 && self.Group == other.Group,
		HeadOn: head && self.Moving && other.Moving, WaitedOut: self.WaitedOut, Cornered: self.Cornered}
	if self.Moving {
		t.OnMyGoal = s.keep.OnGoal(self, other)
		if !other.Moving {
			_, t.Room = s.keep.StepAside(other.Member(), self)
		}
		if self.Minded {
			if s.blocking == nil {
				s.blocking = map[uid.UID64]Blocked{}
			}
			s.blocking[self.ID] = blockedOf(t, cell)
		}
	}
	if hosted {
		s.touches.Dispatch(tick, s.marksOf(self.ID), s.marksOf(other.ID), t)
	}
}

func (s *navigationSystem) marksOf(id uid.UID64) host.Marks {
	if !s.marks.Seek(id) {
		return host.Marks{}
	}
	return s.touches.Read(0, s.marksCols.Row(s.marks.Cursor()), int(s.marks.Cursor().Slot))
}

// way is the way a leaves b
func headOn(a, b bodies.Body, way geom.Vec) bool {
	towards := func(v, dir geom.Vec) bool {
		l := math.Hypot(v.X, v.Y)
		return l > 1e-9 && (v.X*dir.X+v.Y*dir.Y)/l > 0.5
	}
	return a.Moving && b.Moving && towards(a.Vel, geom.NewVec(-way.X, -way.Y)) && towards(b.Vel, way)
}

func (s *navigationSystem) tell(cb *goke.CmdBuf, d time.Duration) {
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

func (s *navigationSystem) carryOut(m bodies.Member, o *MoveOrder, st steering.Helm, d time.Duration) (stop, held bool) {
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

func (s *navigationSystem) stepAside(m bodies.Member, o *MoveOrder, of uid.UID64) {
	other, ok := s.keep.Other(of)
	if !ok {
		return
	}
	aside, ok := s.keep.StepAside(m, other)
	if !ok {
		return
	}
	aside.Linger, aside.Face, aside.Group = spacing.YieldLinger, o.Face, o.Group
	aside.Enqueue(Goal{Cell: o.Target, Spot: o.Spot, At: o.At})
	for _, g := range o.Waypoints[:o.Queued] {
		aside.Enqueue(g)
	}
	if o.Leg.Active {
		aside.Leg = o.Leg
	}
	*o = aside
}

// a unit vector
func (s *navigationSystem) wayTo(m bodies.Member, c cell.ID) geom.Vec {
	have := m.Centre()
	to := s.unwrap(have, s.grid.CellCenter(c))
	d := geom.NewVec(to.X-have.X, to.Y-have.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

func (s *navigationSystem) gather(dst []bodies.Body) []bodies.Body {
	for s.query.Paced(0); s.query.Next(); {
		cursor := s.query.Cursor()
		bases, orders, movers := s.base.Slice(cursor), s.order.Slice(cursor), s.mover.Slice(cursor)
		cells, owned, lasts := s.cell.Slice(cursor), s.owners.Slice(cursor), s.lastOrder.Slice(cursor)
		zs, minded := s.z.Slice(cursor), s.mind.Slice(cursor) != nil
		for i, id := range cursor.IDs {
			pos := bases[i].Pos
			b := bodies.Body{ID: id, At: pos.Center(), Half: geom.NewVec(pos.Size.X/2, pos.Size.Y/2), Vel: bases[i].Vel.Delta(), Domain: unit.DomainAt(movers, i),
				Moving: orders != nil, Cell: cells[i].Cell, Minded: minded, Facing: bases[i].Vel.Dir}
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

func (s *navigationSystem) goal(c cell.ID, spot geom.Vec) geom.Vec {
	if spot == (geom.Vec{}) {
		return s.grid.CellCenter(c)
	}
	return spot
}

// the short way round on a wrapping axis
func (s *navigationSystem) unwrap(have, to geom.Vec) geom.Vec {
	if s.space == nil {
		return to
	}
	return geom.NewVec(
		have.X+spacing.ShortestAxisDelta(have.X, to.X, s.space.Width, s.space.Edges.WrapsX()),
		have.Y+spacing.ShortestAxisDelta(have.Y, to.Y, s.space.Height, s.space.Edges.WrapsY()),
	)
}

func (s *navigationSystem) ahead(have, want geom.Vec, p *Path, waypoint, target cell.ID, end geom.Vec) []geom.Vec {
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

func (s *navigationSystem) drive(m bodies.Member, st steering.Helm, heading, have geom.Vec, route []geom.Vec, reach, speed float64) {
	at := lookahead(have, route, reach)
	if at == have {
		s.keep.Steer(m, st, geom.Vec{}, speed)
		return
	}
	dir := geom.NewVec(at.X-have.X, at.Y-have.Y)
	s.keep.Steer(m, st, dir, speed*turnFactor(heading, dir))
}

// the turning radius at the current speed
func lookaheadReach(st steering.Helm, dt float64) float64 {
	if st.TurnRate <= 0 {
		return 0
	}
	return math.Abs(st.Speed) * dt / st.TurnRate
}

// the share of speed kept through the sharpest turn
const turnCrawl = 0.2

func turnFactor(heading, dir geom.Vec) float64 {
	h, d := math.Hypot(heading.X, heading.Y), math.Hypot(dir.X, dir.Y)
	if h == 0 || d == 0 {
		return 1
	}
	return min(max((heading.X*dir.X+heading.Y*dir.Y)/(h*d), turnCrawl), 1)
}

// the point reach along the polyline have → route[0] → route[1] …, or its end
func lookahead(have geom.Vec, route []geom.Vec, reach float64) geom.Vec {
	if len(route) == 0 {
		return have
	}
	if reach <= 0 {
		return route[0]
	}
	at := have
	for _, p := range route {
		dx, dy := p.X-at.X, p.Y-at.Y
		d := math.Hypot(dx, dy)
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
func passed(have, w, from geom.Vec, reach float64) bool {
	if math.Hypot(have.X-w.X, have.Y-w.Y) <= max(reach, spacing.ArrivalEpsilon) {
		return true
	}
	ax, ay := w.X-from.X, w.Y-from.Y
	if ax == 0 && ay == 0 {
		return false
	}
	return (have.X-w.X)*ax+(have.Y-w.Y)*ay >= 0
}

// never below the speed braking would leave it at the arrival radius within
func approach(st steering.Helm, dist, within float64) float64 {
	brake := st.Braking()
	if brake <= 0 {
		return st.MaxSpeed
	}
	v := max(math.Sqrt(2*brake*dist), math.Sqrt(2*brake*within))
	return min(v, st.MaxSpeed)
}

type refusal struct {
	cell               cell.ID
	deck, held, corner bool
}

// every cell of the step is claimed, or none: an inactive Leg and why
func (s *navigationSystem) reserveLeg(from, to order.Spot, id uid.UID64,
	domain cell.Domain) (Leg, refusal) {
	leg := Leg{From: from.C, To: to.C, FromDeck: from.Deck, ToDeck: to.Deck, Active: true}
	if c1, c2, diag := s.grid.DiagonalNeighbors(from.C, to.C); diag {
		leg.C1, leg.C2, leg.Diagonal, leg.Decked = c1, c2, true, s.decked(from.C, to.C)
	}
	held, n := order.Held(leg)
	for _, h := range held[1:n] {
		if !s.terrain.Kind(h.C).Admits(domain) {
			return Leg{}, refusal{cell: h.C, deck: h.Deck}
		}
		if !s.occupancy.CanEnter(h.C, h.Deck, id, domain) {
			return Leg{}, refusal{cell: h.C, deck: h.Deck, held: true, corner: h.C != to.C}
		}
	}
	for _, h := range held[:n] {
		s.occupancy.Enter(h.C, h.Deck, id, domain)
	}
	return leg, refusal{}
}

func (s *navigationSystem) decked(from, to cell.ID) bool {
	r, ok := s.terrain.(interface{ Reaches(c, n cell.ID) bool })
	return ok && (r.Reaches(from, to) || r.Reaches(to, from))
}

func (s *navigationSystem) admitsAll(cells []cell.ID, domain cell.Domain) bool {
	for _, c := range cells {
		if !s.terrain.Kind(c).Admits(domain) {
			return false
		}
	}
	return true
}

func (s *navigationSystem) admitsLeg(leg Leg, domain cell.Domain) bool {
	held, n := order.Held(leg)
	for _, h := range held[1:n] {
		if !s.terrain.Kind(h.C).Admits(domain) {
			return false
		}
	}
	return true
}

func (s *navigationSystem) releaseLeg(leg Leg, id uid.UID64) {
	held, n := order.Held(leg)
	for _, h := range held[:n] {
		s.occupancy.Leave(h.C, id)
	}
}

func (s *navigationSystem) enter(states []tag.Tags[unit.States], i int, id uid.UID64) {
	if states != nil {
		states[i] = states[i].With(unit.Entered)
	} else {
		s.lacking = append(s.lacking, id)
	}
}

func (s *navigationSystem) ordered(id uid.UID64) bool {
	return s.outside.Seek(id) && s.order.At(s.outside.Cursor()) != nil
}

// after a Populate as after a Load; the cells the units stand on the board holds itself
func (s *navigationSystem) seedLegs() {
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
