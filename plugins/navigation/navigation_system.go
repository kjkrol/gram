package navigation

import (
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// MaxWaypoints bounds the goals queued behind a MoveOrder's Target.
const MaxWaypoints = 8

// MoveOrder commands an entity to path toward Target, then through each queued goal in turn;
// navigationSystem removes it once the last is reached.
type MoveOrder struct {
	Target board.CellID
	// Spot is where in Target the entity's centre stops, zero the centre; At is the point the order
	// was given for, round which a group stands. Only BodySpacing sets them.
	Spot, At  geom.Vec
	Waypoints [MaxWaypoints]Goal // goals after Target, in order
	Queued    uint8              // how many of Waypoints are in use
	Path      Path
	Leg       Leg
	Waited    time.Duration
	// Bumped says the entity struck someone since the last tick; Cooldown is how long it then
	// keeps its new route before it would react to a bump again.
	Bumped   bool
	Cooldown time.Duration
	// Struck is the way off whoever the entity struck last, Hit who that was when HitUnit (else the
	// ground),
	// and Aside the way it steps round them for AsideFor more; Met holds the last Mets it struck
	// standing, whose spots it keeps clear of when it stands elsewhere, and Avoid the first Avoids
	// cells it found them in, which its routes go round. Stalled is how long it has come no nearer
	// Toward, the cell it heads for, than Closest — under CellSpacing, how long its step into
	// Toward has been refused, Held once for too long, Hit who holds it; Stalls how often it
	// stalled since it last reached one.
	Struck, Aside geom.Vec
	Hit           uid.UID64
	HitUnit       bool
	AsideFor      time.Duration
	Met           [4]uid.UID64
	Mets          uint8
	Avoid         [4]board.CellID
	Avoids        uint8
	Stalled       time.Duration
	Toward        board.CellID
	Closest       float64
	Stalls        uint8
	Held          bool
	// Linger is how long the entity stands on its Target before it goes on to the next queued goal;
	// zero passes it. GivingWay marks an order to give way to one on the move — to linger aside,
	// then go home — whom nobody gives way to in turn.
	Linger    time.Duration
	GivingWay bool
	// Holding is how long more a Hold keeps the entity where it stands, the way ahead closed;
	// WaitedOut says one ran out, Cornered that a Detour found no way round — both till its next
	// step.
	Holding   time.Duration
	WaitedOut bool
	Cornered  bool
	// Face is the world point the entity turns towards once it has arrived; zero turns it nowhere.
	Face geom.Vec
	// Group is the MoveTo that gave the order, one number to every unit it sent; zero, none.
	Group uint32
}

// Goal is a goal queued behind a MoveOrder's Target: its Cell, where in it the entity stops
// (Spot; zero, the centre) and the point it was given for (At).
type Goal struct {
	Cell     board.CellID
	Spot, At geom.Vec
}

// Enqueue adds a goal after the last queued one; false when the queue is full.
func (m *MoveOrder) Enqueue(g Goal) bool {
	if int(m.Queued) >= MaxWaypoints {
		return false
	}
	m.Waypoints[m.Queued] = g
	m.Queued++
	return true
}

// advance makes the next queued goal the Target and drops the Path; false with nothing queued.
func (m *MoveOrder) advance() bool {
	if m.Queued == 0 {
		return false
	}
	g := m.Waypoints[0]
	m.Target, m.Spot, m.At = g.Cell, g.Spot, g.At
	copy(m.Waypoints[:], m.Waypoints[1:m.Queued])
	m.Queued--
	m.Path = Path{}
	return true
}

// meet notes id as one struck standing; the oldest goes first.
func (m *MoveOrder) meet(id uid.UID64) {
	for _, known := range m.Met[:m.Mets] {
		if known == id {
			return
		}
	}
	if int(m.Mets) < len(m.Met) {
		m.Met[m.Mets] = id
		m.Mets++
		return
	}
	copy(m.Met[:], m.Met[1:])
	m.Met[len(m.Met)-1] = id
}

// learn notes c as a cell someone stands in, for the routes to go round; the oldest goes first.
func (m *MoveOrder) learn(c board.CellID) {
	for _, known := range m.Avoid[:m.Avoids] {
		if known == c {
			return
		}
	}
	if int(m.Avoids) < len(m.Avoid) {
		m.Avoid[m.Avoids] = c
		m.Avoids++
		return
	}
	copy(m.Avoid[:], m.Avoid[1:])
	m.Avoid[len(m.Avoid)-1] = c
}

// Leg is the single step an entity is travelling: every cell it holds in
// Occupancy until it reaches To's center.
type Leg struct {
	From, To board.CellID
	C1, C2   board.CellID
	Diagonal bool
	Active   bool
}

// cells lists every cell leg holds: From, To, and both corners of a diagonal step.
func (l Leg) cells() []board.CellID {
	if l.Diagonal {
		return []board.CellID{l.From, l.To, l.C1, l.C2}
	}
	return []board.CellID{l.From, l.To}
}

// navigationSystem paths MoveOrder-commanded entities toward their target, asking their Steering
// for a heading at the lookahead point and a speed from their profile.
type navigationSystem struct {
	grid       board.Grid
	terrain    board.Terrain
	occupancy  board.Occupancy
	space      *aabbworld.Space
	pathFinder *pathFinder
	keep       keeping
	bodies     []body // every unit this tick, for the keeping; reused
	// wanted is who came at each standing unit last tick, by a step into its cell refused, and
	// wanting who does this tick: swapped at the end of the tick, so the standing are asked a
	// tick later whatever order the chunks come in
	wanted, wanting map[uid.UID64]press
	presses         []press // reused

	query *goke.Query
	cell  goke.Comp[board.Cell]
	base  goke.Comp[world.Base]
	steer goke.Comp[steering.Steering]
	order goke.OptComp[MoveOrder]
	mover goke.OptComp[board.Mover]
	z     goke.OptComp[world.Z]
	coll  goke.OptComp[collision.Collider]
	hand  goke.OptComp[steering.Driven]
	route []geom.Vec // the centres ahead, unwrapped, reused each entity

	// the units with a tree (plugins/world/act): their minds, whose they are, the
	// facts navigation tells them, the asks they are given and the commands they give themselves
	mind      goke.OptComp[act.Mind]
	owners    goke.OptComp[tag.Tags[owner.Family]]
	blocked   goke.OptComp[Blocked]
	room      goke.OptComp[Room]
	askedWay  goke.OptComp[act.Asked[MakeWay]]
	askedGoal goke.OptComp[act.Asked[FreeGoal]]
	arrived   goke.OptComp[Arrived]
	blockedID goke.CompID
	roomID    goke.CompID
	arrivedID goke.CompID
	blocking  map[uid.UID64]Blocked // who each unit was blocked by this tick
	courtesy  *courtesyQueues
	told      map[uid.UID64]told // what each unit commanded itself this tick
	arrivals  []arrival          // the units with a tree whose order is over this tick
	lookup    *goke.Query        // another unit's order, for a swap
	theirs    goke.Comp[MoveOrder]

	orderID goke.CompID

	arrivedEditor *goke.Editor

	// the units' markers: Entered on for the step a unit's Cell changed — entered of them last
	// step, lacking the ids whose chunk had no family, to get it
	states   goke.OptComp[tag.Tags[States]]
	statesID goke.CompID
	entered  int
	lacking  []uid.UID64

	// terrainSeen is the terrain version every route was last checked against.
	terrainSeen uint64
}

var _ goke.System = (*navigationSystem)(nil)

// targetWaitTimeout is how long an entity waits for an occupied target before settling nearby.
const targetWaitTimeout = 500 * time.Millisecond

// bumpInterval is how long after a bump an entity keeps its new route, deaf to further bumps.
const bumpInterval = 500 * time.Millisecond

// arrivalEpsilon is how close, in world units, counts as having reached the goal.
const arrivalEpsilon = 2.0

// newNavigationSystem builds a navigationSystem over grid; entities move at their Steering profile.
func newNavigationSystem(pathFinder *pathFinder, grid board.Grid, terrain board.Terrain, occupancy board.Occupancy) *navigationSystem {
	return &navigationSystem{grid: grid, terrain: terrain, occupancy: occupancy, pathFinder: pathFinder, keep: newCellKeeping(pathFinder, occupancy)}
}

// withKeeping has the system keep units apart as k says, holding steps in k's occupancy.
func (s *navigationSystem) withKeeping(k keeping) *navigationSystem {
	s.keep, s.occupancy = k, k.occupancy()
	return s
}

// BindSpace attaches the shared spatial index, so arrivals snap to the cell centre.
func (s *navigationSystem) BindSpace(space *aabbworld.Space) { s.space = space }

func (s *navigationSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.base, &s.steer).
		Optional(&s.order).
		Optional(&s.mover).
		Optional(&s.z).
		Optional(&s.coll).
		Optional(&s.hand).
		Optional(&s.mind, &s.owners, &s.blocked, &s.room, &s.askedWay, &s.askedGoal, &s.arrived).
		Optional(&s.states).
		Build()
	s.orderID = si.RegComp[MoveOrder]()
	s.blockedID, s.roomID, s.arrivedID = si.RegComp[Blocked](), si.RegComp[Room](), si.RegComp[Arrived]()
	s.lookup = si.NewQueryBuilder(&s.theirs).Build()
	s.arrivedEditor = s.query.NewEditorBuilder().Remove(goke.Remove[MoveOrder]()).Build()
	s.statesID = si.RegComp[tag.Tags[States]]()
}

func (s *navigationSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	s.clearEntered()
	if s.courtesy != nil {
		if s.told == nil {
			s.told = map[uid.UID64]told{}
		}
		s.courtesy.drain(s.told)
	}
	changed := false
	if v, ok := s.terrain.(interface{ Version() uint64 }); ok && v.Version() != s.terrainSeen {
		s.terrainSeen, changed = v.Version(), true
	}
	s.keep.begin(s.gather)

	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		orders := s.order.Slice(cursor)
		if orders == nil {
			s.giveWay(cb, cursor)
			continue
		}

		cells := s.cell.Slice(cursor)
		bases := s.base.Slice(cursor)
		states := s.states.Slice(cursor)

		var arrivedIDs []uid.UID64

		steers := s.steer.Slice(cursor)
		movers := s.mover.Slice(cursor)
		zs := s.z.Slice(cursor)
		minds, owned, arrived := s.mind.Slice(cursor), s.owners.Slice(cursor), s.arrived.Slice(cursor)
		dt := d.Seconds()

		for i, id := range cursor.IDs {
			domain := board.DomainAt(movers, i)
			o := &orders[i]
			p := &o.Path
			leg := &o.Leg
			st := &steers[i]
			current := cells[i].ID
			actual, ok := s.grid.CellAt(board.Center(bases[i].Pos))
			if !ok {
				actual = current
			}
			m := member{id: id, cell: current, from: current, leg: *leg, domain: domain, pos: bases[i].Pos, vel: bases[i].Vel.Delta(), facing: bases[i].Vel.Dir, order: o}
			if zs != nil {
				m.z = zs[i]
			}
			if movers != nil {
				m.lift = movers[i].Lift
			}
			m.brake = st.Braking()
			if p, ok := s.wanted[id]; ok {
				m.pressedBy, m.pressed = p.other, true
			}
			var mind *act.Mind
			if minds != nil {
				mind, m.minded = &minds[i], true
			}
			if owned != nil {
				m.owners = owned[i]
			}
			if arrived != nil {
				cb.RemoveCompOne(id, s.arrivedID) // on its way again
			}
			arrive := func() {
				arrivedIDs = append(arrivedIDs, id)
				if mind != nil {
					s.arrivals = append(s.arrivals, arrival{id: id, cell: cells[i].ID})
				}
			}

			entered := false
			moveTo := func(c board.CellID) {
				if c == cells[i].ID {
					return
				}
				cells[i].ID = c
				if !entered {
					entered = true
					s.enter(states, i, id)
				}
			}

			if changed && p.Index < p.Length && !s.admitsAll(p.Steps[p.Index:p.Length], domain) {
				p.Length = 0 // the ground changed under the route: plan again
			}

			o.Cooldown = max(o.Cooldown-d, 0)
			if m.minded && o.Bumped && o.HitUnit {
				s.note(m, o, o.Hit, 0, false)
			}
			if o.Bumped {
				o.Bumped = false
				answer := s.keep.bump(m, o)
				if answer == giveUp {
					st.RequestSpeed(0) // stands where it gave up
					arrive()
					continue
				}
				if answer == stopAndPlan {
					// struck someone: stop here, plan again from where it stands, and hold that route
					// for bumpInterval however many bumps follow
					o.Cooldown = bumpInterval
					if leg.Active {
						s.releaseLeg(*leg, id)
						s.occupancy.Enter(actual, id, domain)
					} else if actual != current {
						s.occupancy.Leave(current, id)
						s.occupancy.Enter(actual, id, domain)
					}
					moveTo(actual)
					*leg = Leg{}
					p.Length = 0
					st.RequestSpeed(0)
					continue
				}
			}

			switch {
			case leg.Active && actual == leg.From && !s.admitsAll(leg.cells()[1:], domain):
				// the ground ahead no longer takes the unit: let the leg go and stop
				s.releaseLeg(*leg, id)
				s.occupancy.Enter(actual, id, domain)
				*leg = Leg{}
				p.Length = 0
				st.RequestSpeed(0)
			case leg.Active && !slices.Contains(leg.cells(), actual):
				s.releaseLeg(*leg, id)
				s.occupancy.Enter(actual, id, domain)
				moveTo(actual)
				*leg = Leg{}
				p.Length = 0
			case leg.Active && (actual == leg.From || actual == leg.To):
				moveTo(actual)
			case !leg.Active && actual != current:
				s.occupancy.Leave(current, id)
				s.occupancy.Enter(actual, id, domain)
				moveTo(actual)
				p.Length = 0
			}
			m.cell, m.from, m.leg = cells[i].ID, cells[i].ID, *leg

			if !leg.Active && !s.terrain.Kind(cells[i].ID).Admits(domain) {
				// stuck where it may not be — frozen in, say: the order waits for the ground to change
				st.RequestSpeed(0)
				p.Length = 0
				continue
			}

			if mind != nil && s.act(cursor, i, m, o, st, d) {
				continue // held where it stands by its tree
			}

			s.keep.ready(m, o)
			target := o.Target
			if !leg.Active && (p.Length == 0 || p.Index >= p.Length) && cells[i].ID != target {
				newPath, found := s.keep.route(m, cells[i].ID, o)
				if !found {
					st.RequestSpeed(0)
					o.Waited += d
					dest, destPath, wait, ok := s.keep.lost(m, cells[i].ID, o, o.Waited)
					if wait {
						continue
					}
					if !ok {
						arrive()
						continue
					}
					o.Target, target = dest, dest
					newPath = destPath
				}
				o.Waited = 0
				*p = newPath
			}

			if leg.Active && p.Index < p.Length && p.Steps[p.Index] == leg.From {
				// the route goes back the way the leg came: turn the leg round, same cells held
				leg.From, leg.To = leg.To, leg.From
			}

			waypoint := target
			switch {
			case leg.Active:
				waypoint = leg.To
			case p.Length > 0 && p.Index < p.Length:
				waypoint = p.Steps[p.Index]
			}

			if !leg.Active && waypoint != cells[i].ID {
				reserved, why := s.reserveLeg(cells[i].ID, waypoint, id, domain)
				if !reserved.Active {
					st.RequestSpeed(0)
					switch {
					case !why.held:
						p.Length = 0 // the ground no longer takes the step: plan again
					case why.corner:
						// someone holds a corner of the slantwise step: round them square, not through
						o.learn(why.cell)
						p.Length = 0
					default:
						// someone holds the cell: wait, and ask them off it
						holder, known, asks := s.keep.blocked(m, o, why.cell, d)
						if m.minded && known {
							s.note(m, o, holder, why.cell, true)
						}
						if asks {
							if s.wanting == nil {
								s.wanting = map[uid.UID64]press{}
							}
							s.wanting[holder] = press{other: id, cell: cells[i].ID, way: s.wayBetween(cells[i].ID, why.cell), givingWay: o.GivingWay}
						}
					}
					continue
				}
				*leg = reserved
				o.Holding, o.WaitedOut, o.Cornered = 0, false, false // stepping on
			}

			have := board.Center(bases[i].Pos)
			end := s.goal(target, o.Spot)
			want := s.unwrap(have, s.grid.CellCenter(waypoint))
			if waypoint == target {
				want = s.unwrap(have, end)
			}
			s.keep.watch(m, o, waypoint, want, d)

			reach := lookaheadReach(st, dt)
			if waypoint == target && o.Queued > 0 && o.Linger > 0 {
				// a goal to stand on a while: come to rest on it, wait, then go on as past it
				dx, dy := want.X-have.X, want.Y-have.Y
				if dist, within := math.Hypot(dx, dy), s.keep.within(m); dist > within {
					dir := geom.NewVec(dx, dy)
					s.keep.steer(m, st, dir, approach(st, dist, within)*turnFactor(bases[i].Vel.Dir, dir))
					continue
				}
				st.RequestSpeed(0)
				if leg.Active { // at rest on the goal: the step is done, its cells let go
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(leg.To, id, domain)
					moveTo(leg.To)
					*leg = Leg{}
				}
				if o.Linger = max(o.Linger-d, 0); o.Linger > 0 {
					continue
				}
			}
			if waypoint == target && o.Queued > 0 {
				// a goal with more behind it is passed like a waypoint, then the next one is aimed at
				from := cells[i].ID
				if leg.Active {
					from = leg.From
				}
				if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
					s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end), reach, st.MaxSpeed)
					continue
				}
				if leg.Active && actual != leg.To {
					// passed by the lookahead, not yet entered: hold the leg, aim at the next goal
					next := o.Waypoints[0]
					s.route = append(s.route[:0], s.unwrap(have, s.goal(next.Cell, next.Spot)))
					s.drive(m, st, bases[i].Vel.Dir, have, s.route, reach, st.MaxSpeed)
					continue
				}
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(leg.To, id, domain)
					moveTo(leg.To)
					*leg = Leg{}
				}
				o.advance()
				s.route = append(s.route[:0], s.unwrap(have, s.goal(o.Target, o.Spot)))
				s.drive(m, st, bases[i].Vel.Dir, have, s.route, reach, st.MaxSpeed)
				continue
			}

			if waypoint != target {
				from := cells[i].ID
				if leg.Active {
					from = leg.From
				}
				if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
					s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end), reach, st.MaxSpeed)
					continue
				}
				if leg.Active && actual != leg.To {
					// passed by the lookahead, not yet entered — a wide turner looks further ahead than
					// half a cell: hold the leg and aim beyond it, or the next tick finds the unit
					// short of its cell and plans again
					s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end)[1:], reach, st.MaxSpeed)
					continue
				}
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(leg.To, id, domain)
					moveTo(leg.To)
					*leg = Leg{}
				}
				if p.Index < p.Length && p.Steps[p.Index] == waypoint {
					p.Index++
				}
				// keep going: the next waypoint is aimed at now, reserved next tick
				s.drive(m, st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target, end)[1:], reach, st.MaxSpeed)
				continue
			}

			dx, dy := want.X-have.X, want.Y-have.Y
			dist := math.Hypot(dx, dy)
			if within := s.keep.within(m); dist > within {
				dir := geom.NewVec(dx, dy)
				s.keep.steer(m, st, dir, approach(st, dist, within)*turnFactor(bases[i].Vel.Dir, dir))
				continue
			}

			st.RequestSpeed(0)
			st.Speed = 0
			if face := o.Face; face != (geom.Vec{}) {
				st.Request(geom.NewVec(face.X-want.X, face.Y-want.Y)) // turned by the steering, in place
			}

			if s.space != nil && (dx != 0 || dy != 0) {
				s.space.Move(&bases[i].Pos.AABB, geom.NewVec(dx, dy))
			}

			if leg.Active {
				s.releaseLeg(*leg, id)
				s.occupancy.Enter(leg.To, id, domain)
				moveTo(leg.To)
				*leg = Leg{}
			}
			if at, ok := s.grid.CellAt(want); ok && at != cells[i].ID {
				// put on its spot in another cell than it stood in a moment ago
				s.occupancy.Leave(cells[i].ID, id)
				s.occupancy.Enter(at, id, domain)
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
		for _, id := range s.lacking { // after the chunk's own changes: a move by id
			cb.AddOne(id, s.statesID, tag.Tags[States](0).With(Entered))
		}
		s.lacking = s.lacking[:0]
	}
	s.tell(cb, d)
	for _, a := range s.arrivals {
		cb.AddOne(a.id, s.arrivedID, Arrived{Cell: a.cell})
	}
	s.arrivals = s.arrivals[:0]
	clear(s.told)
	s.wanted, s.wanting = s.wanting, s.wanted
	clear(s.wanting)
}

// arrival is a unit with a tree whose order is over, standing on cell.
type arrival struct {
	id   uid.UID64
	cell board.CellID
}

// wayBetween is the way from cell a's centre to cell b's, a unit vector, the short way round.
func (s *navigationSystem) wayBetween(a, b board.CellID) geom.Vec {
	from := s.grid.CellCenter(a)
	to := s.unwrap(from, s.grid.CellCenter(b))
	d := geom.NewVec(to.X-from.X, to.Y-from.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

// giveWay has every unit standing in cursor's chunk, no hand on it, that one on the move came at
// — striking it, or refused a step into its cell — give way as the keeping says; one with a tree is told instead whether it has room when asked, and steps aside when its tree says so.
func (s *navigationSystem) giveWay(cb *goke.CmdBuf, cursor *goke.Cursor) {
	colls, minds := s.coll.Slice(cursor), s.mind.Slice(cursor)
	if colls == nil && minds == nil && len(s.wanted) == 0 {
		return
	}
	cells, bases, movers, zs, hands := s.cell.Slice(cursor), s.base.Slice(cursor), s.mover.Slice(cursor), s.z.Slice(cursor), s.hand.Slice(cursor)
	if hands != nil {
		return
	}
	owned := s.owners.Slice(cursor)
	for i, id := range cursor.IDs {
		m := member{id: id, cell: cells[i].ID, from: cells[i].ID, domain: board.DomainAt(movers, i), pos: bases[i].Pos, vel: bases[i].Vel.Delta(), facing: bases[i].Vel.Dir}
		if zs != nil {
			m.z = zs[i]
		}
		if movers != nil {
			m.lift = movers[i].Lift
		}
		if owned != nil {
			m.owners = owned[i]
		}
		if minds != nil {
			m.minded = true
			s.standing(cb, cursor, i, m)
			continue
		}
		s.presses = s.presses[:0]
		if colls != nil {
			for _, c := range colls[i].Contacts() {
				if !c.Terrain {
					s.presses = append(s.presses, press{other: c.Other, way: c.Normal})
				}
			}
		}
		if p, ok := s.wanted[id]; ok {
			s.presses = append(s.presses, p)
		}
		if len(s.presses) == 0 {
			continue
		}
		if order, ok := s.keep.yield(m, s.presses); ok {
			cb.AddOne(id, s.orderID, order)
		}
	}
}

// standing tells m, standing, with a tree, whether it has room when an ally asks it to
// make way or to free its goal, and carries out the StepAside it gives itself.
func (s *navigationSystem) standing(cb *goke.CmdBuf, cursor *goke.Cursor, i int, m member) {
	var asker uid.UID64
	asked := false
	if a := s.askedWay.Slice(cursor); a != nil {
		asker, asked = a[i].From, true
	} else if a := s.askedGoal.Slice(cursor); a != nil {
		asker, asked = a[i].From, true
	}
	if t, ok := s.told[m.id]; ok && t.aside {
		if b, ok := s.keep.other(t.stepAside.Of); ok {
			if order, ok := s.keep.stepAside(m, b, t.stepAside.Return); ok {
				cb.AddOne(m.id, s.orderID, order)
				return
			}
		}
	}
	rooms := s.room.Slice(cursor)
	if !asked {
		if rooms != nil {
			cb.RemoveCompOne(m.id, s.roomID)
		}
		return
	}
	var r Room
	if b, ok := s.keep.other(asker); !ok || !owner.Allies(m.owners, b.owners) {
		r.Stranger = true
	} else {
		r.Free, r.Ally, r.Beside = s.keep.room(m, b)
	}
	if rooms == nil || rooms[i] != r {
		cb.AddOne(m.id, s.roomID, r)
	}
}

// note has m, with a tree, blocked this tick by who — in cell when inCell, else where they
// stand — for tell to put on it.
func (s *navigationSystem) note(m member, o *MoveOrder, who uid.UID64, cell board.CellID, inCell bool) {
	other, ok := s.keep.other(who)
	if !ok {
		return
	}
	if !inCell {
		cell = other.cell
	}
	b := Blocked{By: who, Cell: cell, Stranger: !owner.Allies(m.owners, other.owners), Moving: other.moving,
		Groupmate: o.Group != 0 && other.group == o.Group, OnMyGoal: s.keep.onGoal(m, o, other), Yielding: o.GivingWay,
		WaitedOut: o.WaitedOut, Cornered: o.Cornered, Lasts: blockedLasts}
	b.First = (m.id < who) != slices.Contains(o.Met[:o.Mets], who)
	if b.Groupmate {
		g := s.grid
		b.Shortens = g.Distance(m.cell, other.goal)+g.Distance(other.cell, o.Target) < g.Distance(m.cell, o.Target)+g.Distance(other.cell, other.goal)-0.5
	}
	if s.blocking == nil {
		s.blocking = map[uid.UID64]Blocked{}
	}
	s.blocking[m.id] = b
}

// blockedLasts is how long Blocked stays on a unit with nobody struck: contacts come and go.
const blockedLasts = 400 * time.Millisecond

// tell puts on every unit with a tree what blocked it this tick, and takes Blocked off
// one no longer blocked once its hold is up — two that were on the move then count as having met,
// so the next time they meet the other one waits.
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
			if orders != nil && f.Moving {
				orders[i].meet(f.By)
			}
			cb.RemoveCompOne(id, s.blockedID)
		}
	}
	clear(s.blocking)
}

// act carries out what m's tree commanded it this tick: a detour is planned — Cornered with no
// way round the one in the way — a swap of goals made, a place beside the goal found, a step aside
// taken before going on, each at once; a Hold keeps m where it stands, true, until the way ahead
// clears or it has held stallAfter — WaitedOut then — while it is blocked.
func (s *navigationSystem) act(cursor *goke.Cursor, i int, m member, o *MoveOrder, st *steering.Steering, d time.Duration) bool {
	facts := s.blocked.Slice(cursor)
	var by Blocked
	if facts != nil {
		by = facts[i]
	}
	if t, ok := s.told[m.id]; ok {
		switch {
		case t.detour && facts != nil:
			if by.Cell != o.Target && by.Cell != m.cell {
				o.learn(by.Cell)
			}
			path, ok := s.keep.route(m, m.from, o)
			o.Holding, o.Cornered = 0, !ok || slices.Contains(path.Steps[path.Index:path.Length], by.Cell)
			if !o.Cornered {
				o.Path = path
			}
		case t.hold:
			o.Holding, o.WaitedOut = stallAfter, false
		case t.swap:
			s.swap(o, t.swapWith)
		case t.settle:
			if other, ok := s.keep.other(t.settleBeside); ok {
				s.keep.settle(m, o, other)
			}
		case t.aside:
			s.stepAside(m, o, t.stepAside.Of)
		}
	}
	if o.Holding <= 0 {
		return false
	}
	next := o.Target
	if o.Path.Index < o.Path.Length {
		next = o.Path.Steps[o.Path.Index]
	}
	if facts == nil || s.keep.mayStep(m, next, s.wayTo(m, next)) {
		o.Holding = 0 // the way ahead is clear
		return false
	}
	st.RequestSpeed(0)
	if o.Holding -= d; o.Holding <= 0 {
		o.Holding, o.WaitedOut = 0, true
	}
	s.note(m, o, by.By, by.Cell, true) // still blocked: the fact holds
	return true
}

// stepAside has m, on the move, step off the way of of a while, then go on to its own goals as they
// were; nothing with nowhere to step.
func (s *navigationSystem) stepAside(m member, o *MoveOrder, of uid.UID64) {
	other, ok := s.keep.other(of)
	if !ok {
		return
	}
	aside, ok := s.keep.stepAside(m, other, false)
	if !ok {
		return
	}
	aside.Linger, aside.Face, aside.Group = yieldLinger, o.Face, o.Group
	aside.Enqueue(Goal{Cell: o.Target, Spot: o.Spot, At: o.At})
	for _, g := range o.Waypoints[:o.Queued] {
		aside.Enqueue(g)
	}
	if o.Leg.Active {
		aside.Leg = o.Leg
	}
	*o = aside
}

// wayTo is the way from m towards cell c's centre, a unit vector.
func (s *navigationSystem) wayTo(m member, c board.CellID) geom.Vec {
	have := m.centre()
	to := s.unwrap(have, s.grid.CellCenter(c))
	d := geom.NewVec(to.X-have.X, to.Y-have.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

// swap swaps o's goal with with's, both of one group; false otherwise.
func (s *navigationSystem) swap(o *MoveOrder, with uid.UID64) bool {
	if o.Group == 0 || !s.lookup.Seek(with) {
		return false
	}
	t := s.theirs.At(s.lookup.Cursor())
	if t == nil || t.Group != o.Group {
		return false
	}
	o.Target, t.Target = t.Target, o.Target
	o.Spot, t.Spot = t.Spot, o.Spot
	o.At, t.At = t.At, o.At
	o.Path, t.Path = Path{}, Path{}
	return true
}

// gather appends every unit as it stands to dst.
func (s *navigationSystem) gather(dst []body) []body {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, orders, movers := s.base.Slice(cursor), s.order.Slice(cursor), s.mover.Slice(cursor)
		cells, owned := s.cell.Slice(cursor), s.owners.Slice(cursor)
		for i, id := range cursor.IDs {
			pos := bases[i].Pos
			b := body{id: id, at: board.Center(pos), half: geom.NewVec(pos.Size.X/2, pos.Size.Y/2), vel: bases[i].Vel.Delta(), domain: board.DomainAt(movers, i), moving: orders != nil, cell: cells[i].ID}
			if orders != nil {
				b.yielding, b.group, b.goal = orders[i].GivingWay, orders[i].Group, orders[i].Target
			}
			if owned != nil {
				b.owners = owned[i]
			}
			dst = append(dst, b)
		}
	}
	return dst
}

// goal is where in c an entity stops: spot, or the centre where spot is zero.
func (s *navigationSystem) goal(c board.CellID, spot geom.Vec) geom.Vec {
	if spot == (geom.Vec{}) {
		return s.grid.CellCenter(c)
	}
	return spot
}

// unwrap is to as seen from have: the short way round on a wrapping axis.
func (s *navigationSystem) unwrap(have, to geom.Vec) geom.Vec {
	if s.space == nil {
		return to
	}
	return geom.NewVec(
		have.X+shortestAxisDelta(have.X, to.X, s.space.Width, s.space.Edges.WrapsX()),
		have.Y+shortestAxisDelta(have.Y, to.Y, s.space.Height, s.space.Edges.WrapsY()),
	)
}

// ahead is the route to steer by: want, then the centres of the steps after waypoint, up to
// target, which ends at end.
func (s *navigationSystem) ahead(have, want geom.Vec, p *Path, waypoint, target board.CellID, end geom.Vec) []geom.Vec {
	s.route = append(s.route[:0], want)
	if waypoint == target {
		return s.route
	}
	k := int(p.Index)
	if k < int(p.Length) && p.Steps[k] == waypoint {
		k++
	}
	last := want
	for ; k < int(p.Length) && len(s.route) < maxAhead; k++ {
		c := s.grid.CellCenter(p.Steps[k])
		if p.Steps[k] == target {
			c = end
		}
		last = s.unwrap(last, c)
		s.route = append(s.route, last)
	}
	return s.route
}

// maxAhead caps how many centres ahead the lookahead point is sought along.
const maxAhead = 8

// drive asks m's st, through the keeping, for the heading to the lookahead point on route and for
// speed, the less the sharper the turn.
func (s *navigationSystem) drive(m member, st *steering.Steering, heading, have geom.Vec, route []geom.Vec, reach, speed float64) {
	at := lookahead(have, route, reach)
	if at == have {
		s.keep.steer(m, st, geom.Vec{}, speed)
		return
	}
	dir := geom.NewVec(at.X-have.X, at.Y-have.Y)
	s.keep.steer(m, st, dir, speed*turnFactor(heading, dir))
}

// lookaheadReach is the turning radius at the current speed: how far ahead to look.
func lookaheadReach(st *steering.Steering, dt float64) float64 {
	if st.TurnRate <= 0 {
		return 0
	}
	return math.Abs(st.Speed) * dt / st.TurnRate
}

// turnCrawl is the share of speed kept through the sharpest turn.
const turnCrawl = 0.2

// turnFactor scales speed by how far dir is from heading: straight on keeps it, a U-turn crawls.
func turnFactor(heading, dir geom.Vec) float64 {
	h, d := math.Hypot(heading.X, heading.Y), math.Hypot(dir.X, dir.Y)
	if h == 0 || d == 0 {
		return 1
	}
	return min(max((heading.X*dir.X+heading.Y*dir.Y)/(h*d), turnCrawl), 1)
}

// lookahead is the point reach along the polyline have → route[0] → route[1] …, or its end.
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

// passed reports w left behind: have beyond the plane through w perpendicular to from → w, or
// within reach of w, where the lookahead already looks past it.
func passed(have, w, from geom.Vec, reach float64) bool {
	if math.Hypot(have.X-w.X, have.Y-w.Y) <= max(reach, arrivalEpsilon) {
		return true
	}
	ax, ay := w.X-from.X, w.Y-from.Y
	if ax == 0 && ay == 0 {
		return false
	}
	return (have.X-w.X)*ax+(have.Y-w.Y)*ay >= 0
}

// approach is the speed that brings st to rest on the goal dist away, never below the speed
// braking would leave it at the arrival radius within.
func approach(st *steering.Steering, dist, within float64) float64 {
	brake := st.Braking()
	if brake <= 0 {
		return st.MaxSpeed
	}
	v := max(math.Sqrt(2*brake*dist), math.Sqrt(2*brake*within))
	return min(v, st.MaxSpeed)
}

// refusal is why a step could not be reserved: the ground, or a cell someone holds — the step's
// own cell, or a corner of a slantwise one.
type refusal struct {
	cell         board.CellID
	held, corner bool
}

// reserveLeg claims every cell a step from→to can touch, or none of them: an inactive Leg and why.
func (s *navigationSystem) reserveLeg(from, to board.CellID, id uid.UID64, domain board.Domain) (Leg, refusal) {
	leg := Leg{From: from, To: to, Active: true}
	if c1, c2, diag := s.grid.DiagonalNeighbors(from, to); diag {
		leg.C1, leg.C2, leg.Diagonal = c1, c2, true
	}
	for _, c := range leg.cells()[1:] {
		if !s.terrain.Kind(c).Admits(domain) {
			return Leg{}, refusal{cell: c}
		}
		if !s.occupancy.CanEnter(c, id, domain) {
			return Leg{}, refusal{cell: c, held: true, corner: c != to}
		}
	}
	for _, c := range leg.cells() {
		s.occupancy.Enter(c, id, domain)
	}
	return leg, refusal{}
}

// admitsAll reports whether every cell still admits domain.
func (s *navigationSystem) admitsAll(cells []board.CellID, domain board.Domain) bool {
	for _, c := range cells {
		if !s.terrain.Kind(c).Admits(domain) {
			return false
		}
	}
	return true
}

// releaseLeg gives up every cell leg holds.
func (s *navigationSystem) releaseLeg(leg Leg, id uid.UID64) {
	for _, c := range leg.cells() {
		s.occupancy.Leave(c, id)
	}
}

func shortestAxisDelta(have, want float64, size uint32, wraps bool) float64 {
	d := want - have
	if !wraps || size == 0 {
		return d
	}
	s := float64(size)
	if d > s/2 {
		d -= s
	} else if d < -s/2 {
		d += s
	}
	return d
}

// clearEntered has Entered off on every unit, when any had it on last step.
func (s *navigationSystem) clearEntered() {
	if s.entered == 0 {
		return
	}
	s.entered = 0
	for s.query.All(); s.query.Next(); {
		states := s.states.Slice(s.query.Cursor())
		for i := range states {
			states[i] = states[i].Without(Entered)
		}
	}
}
