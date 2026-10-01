package navigation

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// MaxWaypoints bounds the goals queued behind a MoveOrder's Target.
const MaxWaypoints = 8

// MoveOrder commands an entity to path toward Target, then through each queued goal in turn;
// navigationSystem removes it once the last is reached.
type MoveOrder struct {
	Target cell.ID
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
	Avoid         [4]cell.ID
	Avoids        uint8
	Stalled       time.Duration
	Toward        cell.ID
	Closest       float64
	Stalls        uint8
	Held          bool
	// Linger is how long the entity stands on its Target before it goes on to the next queued goal;
	// zero passes it. GivingWay marks an order to give way (StepAside), whom nobody gives way to in
	// turn and whose Target is drawn as no goal.
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
	// Round is a patrol's: with goals in it the order never ends — reached or given up, it goes on
	// to the round's next goal (Patrol).
	Round Round
}

// Round is a patrol: Goals walked to in turn and round again, for ever, Pause stood on each one
// reached; Next is the goal after the Target, Stood how long the entity has stood on it.
type Round struct {
	Goals       [MaxWaypoints]cell.ID
	Count, Next uint8
	Pause       time.Duration
	Stood       time.Duration
}

// Patrol is the order to walk to cells in turn and round again, for ever, standing pause on each:
// a guard's round, a wanderer's walk. A kind gives it its units (comp.Load), each its own round.
func Patrol(pause time.Duration, cells ...cell.ID) MoveOrder {
	if len(cells) == 0 || len(cells) > MaxWaypoints {
		panic(fmt.Sprintf("navigation: a patrol of %d goals, want 1 to %d", len(cells), MaxWaypoints))
	}
	o := MoveOrder{Target: cells[0], Round: Round{Count: uint8(len(cells)), Next: uint8(1 % len(cells)), Pause: pause}}
	copy(o.Round.Goals[:], cells)
	return o
}

// goOn aims the order at its round's next goal; false for an order with no round.
func (m *MoveOrder) goOn() bool {
	r := &m.Round
	if r.Count == 0 {
		return false
	}
	m.Target, m.Spot, m.At, m.Path = r.Goals[r.Next], geom.Vec{}, geom.Vec{}, Path{}
	r.Next, r.Stood = (r.Next+1)%r.Count, 0
	return true
}

// Goal is a goal queued behind a MoveOrder's Target: its Cell, where in it the entity stops
// (Spot; zero, the centre) and the point it was given for (At).
type Goal struct {
	Cell     cell.ID
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
func (m *MoveOrder) learn(c cell.ID) {
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
	From, To cell.ID
	C1, C2   cell.ID
	Diagonal bool
	Active   bool
}

// cells lists every cell leg holds: From, To, and both corners of a diagonal step.
func (l Leg) cells() []cell.ID {
	if l.Diagonal {
		return []cell.ID{l.From, l.To, l.C1, l.C2}
	}
	return []cell.ID{l.From, l.To}
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
	// wanted is who came at each unit last tick, by a step into its cell refused, and wanting who
	// does this tick: swapped at the end of the tick, so two coming at each other's cells know it
	// whatever order the chunks come in
	wanted, wanting map[uid.UID64]uid.UID64

	query  *goke.Query
	cell   goke.Comp[board.At]
	base   goke.Comp[world.Base]
	steer  goke.Comp[steering.Steering]
	course goke.Comp[steering.Course]
	order  goke.OptComp[MoveOrder]
	mover  goke.OptComp[board.Mover]
	z      goke.OptComp[world.Z]
	coll   goke.OptComp[collision.Collider]
	hand   goke.OptComp[steering.Driven]
	route  []geom.Vec // the centres ahead, unwrapped, reused each entity

	// whose the units are, the group each came to the end of last, and for the units with a tree
	// (plugins/world/rule) their minds and the facts navigation tells them
	owners      goke.OptComp[tag.Tags[owner.Family]]
	lastOrder   goke.OptComp[LastOrder]
	lastOrderID goke.CompID
	lastLacking []LastOrder // the units lacking LastOrder whose order is over this chunk, by Group
	lastIDs     []uid.UID64
	mind        goke.OptComp[rule.Mind]
	blocked     goke.OptComp[Blocked]
	arrived     goke.OptComp[Arrived]
	blockedID   goke.CompID
	arrivedID   goke.CompID
	blocking    map[uid.UID64]Blocked // who each unit with a tree was blocked by this tick
	arrivals    []arrival             // the units with a tree whose order is over this tick

	// the commands units give themselves, what each did this tick, and the touches this tick
	// handed to the rules navigation hosts, whose tag families a query of its own reads
	given    *givenQueues
	told     map[uid.UID64]told
	touched  []touching
	felt     map[[2]uid.UID64]bool // the pairs touching this tick, each once; refused, one way
	touches  *host.PairHost[Touch]
	marks    *goke.Query
	markCell goke.Comp[board.At]
	tick     plugin.TickSource // the world's, for the rules

	orderID goke.CompID

	arrivedEditor *goke.Editor

	// the units' markers: Entered on for the step a unit's At changed — entered of them last
	// step, lacking the ids whose chunk had no family, to get it
	states   goke.OptComp[tag.Tags[States]]
	statesID goke.CompID
	entered  int
	lacking  []uid.UID64

	// terrainSeen is the terrain version every route was last checked against.
	terrainSeen uint64
}

var _ goke.System = (*navigationSystem)(nil)

// targetWaitTimeout is how long an entity waits for a target no route reaches before settling
// nearby.
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
	s.query = si.NewQueryBuilder(&s.cell, &s.base, &s.steer, &s.course).
		Optional(&s.order).
		Optional(&s.mover).
		Optional(&s.z).
		Optional(&s.coll).
		Optional(&s.hand).
		Optional(&s.owners, &s.lastOrder).
		Optional(&s.mind, &s.blocked, &s.arrived).
		Optional(&s.states).
		Build()
	s.orderID = si.RegComp[MoveOrder]()
	s.blockedID, s.arrivedID, s.lastOrderID = si.RegComp[Blocked](), si.RegComp[Arrived](), si.RegComp[LastOrder]()
	s.arrivedEditor = s.query.NewEditorBuilder().Remove(goke.Remove[MoveOrder]()).Build()
	s.statesID = si.RegComp[tag.Tags[States]]()
	if s.touches != nil {
		marks := si.NewQueryBuilder(&s.markCell)
		s.touches.Bind(marks)
		s.marks = marks.Build()
	}
}

func (s *navigationSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	s.clearEntered()
	changed := false
	if v, ok := s.terrain.(interface{ Version() uint64 }); ok && v.Version() != s.terrainSeen {
		s.terrainSeen, changed = v.Version(), true
	}
	s.keep.begin(s.gather)
	s.touched = s.touched[:0]
	if s.felt == nil {
		s.felt = map[[2]uid.UID64]bool{}
	}
	if s.keep.byContact() {
		// the boxes that met: their rules' commands are carried out this very tick
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

	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		orders := s.order.Slice(cursor)
		if orders == nil {
			s.standing(cb, cursor)
			continue
		}

		cells := s.cell.Slice(cursor)
		bases := s.base.Slice(cursor)
		states := s.states.Slice(cursor)

		var arrivedIDs []uid.UID64

		steers, courses := s.steer.Slice(cursor), s.course.Slice(cursor)
		movers := s.mover.Slice(cursor)
		zs := s.z.Slice(cursor)
		minds, owned, arrived, lasts := s.mind.Slice(cursor), s.owners.Slice(cursor), s.arrived.Slice(cursor), s.lastOrder.Slice(cursor)
		dt := d.Seconds()

		for i, id := range cursor.IDs {
			domain := board.DomainAt(movers, i)
			o := &orders[i]
			p := &o.Path
			leg := &o.Leg
			st := steering.Helm{Steering: &steers[i], Course: &courses[i]}
			current := cells[i].Cell
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
			if by, ok := s.wanted[id]; ok {
				m.pressedBy, m.pressed = by, true
			}
			if owned != nil {
				m.owners = owned[i]
			}
			if arrived != nil {
				cb.RemoveCompOne(id, s.arrivedID) // on its way again
			}
			arrive := func() {
				if o.goOn() { // a patrol goes on round, never over
					return
				}
				arrivedIDs = append(arrivedIDs, id)
				switch {
				case o.Group == 0: // an order to give way, or to look: the group stays
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
				if !entered {
					entered = true
					s.enter(states, i, id)
				}
			}

			if changed && p.Index < p.Length && !s.admitsAll(p.Steps[p.Index:p.Length], domain) {
				p.Length = 0 // the ground changed under the route: plan again
			}

			o.Cooldown = max(o.Cooldown-d, 0)
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
			m.cell, m.from, m.leg = cells[i].Cell, cells[i].Cell, *leg

			if !leg.Active && !s.terrain.Kind(cells[i].Cell).Admits(domain) {
				// stuck where it may not be — frozen in, say: the order waits for the ground to change
				st.RequestSpeed(0)
				p.Length = 0
				continue
			}

			stop, held := s.carryOut(m, o, st, d)
			if stop {
				st.RequestSpeed(0) // stands where it is
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(actual, id, domain)
					moveTo(actual)
					*leg = Leg{}
				}
				arrive()
				continue
			}
			if held {
				continue // held where it stands by its own command
			}

			s.keep.ready(m, o)
			target := o.Target
			if !leg.Active && (p.Length == 0 || p.Index >= p.Length) && cells[i].Cell != target {
				newPath, found := s.keep.route(m, cells[i].Cell, o)
				if !found {
					st.RequestSpeed(0)
					o.Waited += d
					dest, destPath, wait, ok := s.keep.lost(m, cells[i].Cell, o, o.Waited)
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

			if !leg.Active && waypoint != cells[i].Cell {
				reserved, why := s.reserveLeg(cells[i].Cell, waypoint, id, domain)
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
						// someone holds the cell: the two touch, as a refused step tells
						if holder, known := s.keep.blocked(m, o, why.cell, d); known {
							s.touched = append(s.touched, touching{self: id, other: holder, way: s.wayBetween(why.cell, cells[i].Cell),
								cell: why.cell, from: cells[i].Cell, headOn: m.pressed && m.pressedBy == holder, refused: true})
							if s.wanting == nil {
								s.wanting = map[uid.UID64]uid.UID64{}
							}
							s.wanting[holder] = id
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
				from := cells[i].Cell
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
				from := cells[i].Cell
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
			if at, ok := s.grid.CellAt(want); ok && at != cells[i].Cell {
				// put on its spot in another cell than it stood in a moment ago
				s.occupancy.Leave(cells[i].Cell, id)
				s.occupancy.Enter(at, id, domain)
				moveTo(at)
			}

			if p.Index < p.Length && p.Steps[p.Index] == waypoint {
				p.Index++
			}

			if r := &o.Round; r.Count > 0 && r.Stood < r.Pause {
				r.Stood += d // a patrol stands its pause on the goal it reached
				continue
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

// arrival is a unit with a tree whose order is over, standing on cell.
type arrival struct {
	id   uid.UID64
	cell cell.ID
}

// wayBetween is the way from cell a's centre to cell b's, a unit vector, the short way round.
func (s *navigationSystem) wayBetween(a, b cell.ID) geom.Vec {
	from := s.grid.CellCenter(a)
	to := s.unwrap(from, s.grid.CellCenter(b))
	d := geom.NewVec(to.X-from.X, to.Y-from.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

// standing carries out the StepAside every unit standing in cursor's chunk, no hand on it, gave
// itself: an order aside, where the keeping finds it safe.
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
		of, ok := s.keep.other(t.asideOf)
		if !ok {
			continue
		}
		m := member{id: id, cell: cells[i].Cell, from: cells[i].Cell, domain: board.DomainAt(movers, i), pos: bases[i].Pos, vel: bases[i].Vel.Delta(), facing: bases[i].Vel.Dir}
		if zs != nil {
			m.z = zs[i]
		}
		if movers != nil {
			m.lift = movers[i].Lift
		}
		if order, ok := s.keep.stepAside(m, of); ok {
			cb.AddOne(id, s.orderID, order)
		}
	}
}

// touching is two units touching this tick, a touch of both: self leaving other along way; refused,
// self was refused a step from its cell from into cell, which other holds.
type touching struct {
	self, other uid.UID64
	way         geom.Vec
	cell, from  cell.ID
	headOn      bool
	refused     bool
}

// feel lists the units touching this tick: the contacts their Colliders recorded, each pair once —
// the collision records a contact on the one that struck, or on both.
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
				if c.Terrain || c.Other == 0 {
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

// touch hands every touch of this tick to the rules of Touch, seen from each of the two, and
// notes what blocks every unit on the move with a tree.
func (s *navigationSystem) touch(cb *goke.CmdBuf, d time.Duration) {
	hosted := s.touches != nil && !s.touches.Empty()
	tick := s.tick.Of(cb, d)
	clear(s.felt)
	for _, t := range s.touched {
		if t.refused {
			s.felt[[2]uid.UID64{t.self, t.other}] = true
		}
	}
	for _, t := range s.touched {
		self, ok := s.keep.other(t.self)
		other, found := s.keep.other(t.other)
		if !ok || !found || self.domain&other.domain == 0 {
			continue
		}
		cell, from := other.cell, self.cell
		if t.refused {
			cell, from = t.cell, t.from
		}
		head := t.headOn || t.refused && s.felt[[2]uid.UID64{t.other, t.self}] || !t.refused && headOn(self, other, t.way)
		s.fire(tick, hosted, self, other, t.way, cell, head)
		s.fire(tick, hosted, other, self, geom.NewVec(-t.way.X, -t.way.Y), from, head)
	}
}

// fire hands the Touch of self by other, holding cell, to the rules, and notes it for self when
// it has a tree.
func (s *navigationSystem) fire(tick plugin.Tick, hosted bool, self, other body, way geom.Vec, cell cell.ID, head bool) {
	t := Touch{Self: self.id, Other: other.id, Way: way, Moving: self.moving, OtherMoving: other.moving,
		GivingWay: self.givingWay, OtherGivingWay: other.givingWay, LastGoal: self.lastGoal,
		Ally: owner.Allies(self.owners, other.owners), Groupmate: self.group != 0 && self.group == other.group,
		HeadOn: head && self.moving && other.moving, WaitedOut: self.waitedOut, Cornered: self.cornered}
	if self.moving {
		t.OnMyGoal = s.keep.onGoal(self, other)
		if !other.moving {
			_, t.Room = s.keep.stepAside(other.member(), self)
		}
		if self.minded {
			if s.blocking == nil {
				s.blocking = map[uid.UID64]Blocked{}
			}
			s.blocking[self.id] = blockedOf(t, cell)
		}
	}
	if hosted {
		s.touches.Dispatch(tick, s.marksOf(self.id), s.marksOf(other.id), t)
	}
}

// marksOf is what id carries of the tag families the rules of Touch name.
func (s *navigationSystem) marksOf(id uid.UID64) plugin.Marks {
	if !s.marks.Seek(id) {
		return plugin.Marks{}
	}
	return s.touches.At(0, s.marks.Cursor())
}

// headOn reports whether a and b, both on the move, come at each other: a towards b against way,
// the way a leaves b, and b towards a along it.
func headOn(a, b body, way geom.Vec) bool {
	towards := func(v, dir geom.Vec) bool {
		l := math.Hypot(v.X, v.Y)
		return l > 1e-9 && (v.X*dir.X+v.Y*dir.Y)/l > 0.5
	}
	return a.moving && b.moving && towards(a.vel, geom.NewVec(-way.X, -way.Y)) && towards(b.vel, way)
}

// tell puts on every unit with a tree what blocked it this tick, and takes Blocked off one no
// longer blocked once its hold is up.
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

// carryOut does what m, under o, commanded itself this tick: a Stop ends the order — stop — a step
// aside, a detour and a place beside the goal are taken at once, a detour that makes no headway
// ends the order too; a Hold keeps m where it stands — held — until the way ahead clears or it has
// held stallAfter, WaitedOut then.
func (s *navigationSystem) carryOut(m member, o *MoveOrder, st steering.Helm, d time.Duration) (stop, held bool) {
	if t, ok := s.told[m.id]; ok {
		switch {
		case t.stop:
			return true, false
		case t.aside:
			s.stepAside(m, o, t.asideOf)
		case t.detour:
			if other, ok := s.keep.other(t.detourOf); ok && s.keep.detour(m, o, other) == giveUp {
				return true, false
			}
		case t.pass:
			if other, ok := s.keep.other(t.passOf); ok {
				s.keep.pass(m, o, other)
			}
		case t.hold:
			if o.Holding <= 0 && !o.WaitedOut {
				o.Holding = stallAfter
			}
		case t.settle:
			if other, ok := s.keep.other(t.settleBeside); ok {
				s.keep.settle(m, o, other)
			}
		}
	}
	if o.Holding <= 0 {
		return false, false
	}
	next := o.Target
	if o.Path.Index < o.Path.Length {
		next = o.Path.Steps[o.Path.Index]
	}
	if s.keep.mayStep(m, next, s.wayTo(m, next)) {
		o.Holding = 0 // the way ahead is clear
		return false, false
	}
	st.RequestSpeed(0)
	if o.Holding -= d; o.Holding <= 0 {
		o.Holding, o.WaitedOut = 0, true
	}
	return false, true
}

// stepAside has m, on the move, step off the way of of a while, then go on to its own goals as they
// were; nothing with nowhere to step.
func (s *navigationSystem) stepAside(m member, o *MoveOrder, of uid.UID64) {
	other, ok := s.keep.other(of)
	if !ok {
		return
	}
	aside, ok := s.keep.stepAside(m, other)
	if !ok {
		return
	}
	aside.Linger, aside.Face, aside.Group, aside.Round = yieldLinger, o.Face, o.Group, o.Round
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
func (s *navigationSystem) wayTo(m member, c cell.ID) geom.Vec {
	have := m.centre()
	to := s.unwrap(have, s.grid.CellCenter(c))
	d := geom.NewVec(to.X-have.X, to.Y-have.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return d
}

// gather appends every unit as it stands to dst.
func (s *navigationSystem) gather(dst []body) []body {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, orders, movers := s.base.Slice(cursor), s.order.Slice(cursor), s.mover.Slice(cursor)
		cells, owned, lasts := s.cell.Slice(cursor), s.owners.Slice(cursor), s.lastOrder.Slice(cursor)
		zs, minded := s.z.Slice(cursor), s.mind.Slice(cursor) != nil
		for i, id := range cursor.IDs {
			pos := bases[i].Pos
			b := body{id: id, at: board.Center(pos), half: geom.NewVec(pos.Size.X/2, pos.Size.Y/2), vel: bases[i].Vel.Delta(), domain: board.DomainAt(movers, i),
				moving: orders != nil, cell: cells[i].Cell, minded: minded, facing: bases[i].Vel.Dir}
			if zs != nil {
				b.z = zs[i]
			}
			if movers != nil {
				b.lift = movers[i].Lift
			}
			if lasts != nil {
				b.group = lasts[i].Group
			}
			if orders != nil {
				o := &orders[i]
				b.givingWay, b.goal, b.spot, b.lastGoal = o.GivingWay, o.Target, s.goal(o.Target, o.Spot), o.Queued == 0
				b.waitedOut, b.cornered = o.WaitedOut, o.Cornered
				if o.Group != 0 {
					b.group = o.Group
				}
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
func (s *navigationSystem) goal(c cell.ID, spot geom.Vec) geom.Vec {
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
func (s *navigationSystem) ahead(have, want geom.Vec, p *Path, waypoint, target cell.ID, end geom.Vec) []geom.Vec {
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
func (s *navigationSystem) drive(m member, st steering.Helm, heading, have geom.Vec, route []geom.Vec, reach, speed float64) {
	at := lookahead(have, route, reach)
	if at == have {
		s.keep.steer(m, st, geom.Vec{}, speed)
		return
	}
	dir := geom.NewVec(at.X-have.X, at.Y-have.Y)
	s.keep.steer(m, st, dir, speed*turnFactor(heading, dir))
}

// lookaheadReach is the turning radius at the current speed: how far ahead to look.
func lookaheadReach(st steering.Helm, dt float64) float64 {
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
func approach(st steering.Helm, dist, within float64) float64 {
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
	cell         cell.ID
	held, corner bool
}

// reserveLeg claims every cell a step from→to can touch, or none of them: an inactive Leg and why.
func (s *navigationSystem) reserveLeg(from, to cell.ID, id uid.UID64, domain cell.Domain) (Leg, refusal) {
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
func (s *navigationSystem) admitsAll(cells []cell.ID, domain cell.Domain) bool {
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
