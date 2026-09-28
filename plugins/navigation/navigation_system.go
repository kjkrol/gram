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
	"github.com/kjkrol/gram/plugins/world"
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
	// Struck is the way off whoever the entity struck last, Hit who that was (zero: the ground),
	// and Aside the way it steps round them for AsideFor more; Met holds the last Mets it struck
	// standing, whose spots it keeps clear of when it stands elsewhere, and Avoid the first Avoids
	// cells it found them in, which its routes go round. Stalled is how long it has come no nearer
	// Toward, the cell it heads for, than Closest; Stalls how often it stalled since it last reached
	// one. BodySpacing only.
	Struck, Aside geom.Vec
	Hit           uid.UID64
	AsideFor      time.Duration
	Met           [4]uid.UID64
	Mets          uint8
	Avoid         [4]board.CellID
	Avoids        uint8
	Stalled       time.Duration
	Toward        board.CellID
	Closest       float64
	Stalls        uint8
	// Linger is how long the entity stands on its Target before it goes on to the next queued goal;
	// zero passes it. GivingWay marks an order to give way to one on the move — to linger aside,
	// then go home — whom nobody gives way to in turn.
	Linger    time.Duration
	GivingWay bool
	// Face is the world point the entity turns towards once it has arrived; zero turns it nowhere.
	Face geom.Vec
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

// CellEntered is a one-tick tag added the tick an entity's Cell changes —
// query it to react to a unit stepping onto a cell.
type CellEntered struct{ ID board.CellID }

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

	orderID goke.CompID

	cellEnteredAdd goke.Comp[CellEntered]
	enterVM        *goke.ValueEditor
	arrivedEditor  *goke.Editor
	arrivedVM      *goke.ValueEditor

	enteredQuery     *goke.Query
	cellEnteredClear goke.Comp[CellEntered]
	clearEditor      *goke.Editor

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
	return &navigationSystem{grid: grid, terrain: terrain, occupancy: occupancy, pathFinder: pathFinder, keep: newCellKeeping(pathFinder)}
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
		Build()
	s.orderID = si.RegComp[MoveOrder]()
	s.arrivedEditor = s.query.NewEditorBuilder().Remove(goke.Remove[MoveOrder]()).Build()
	s.enterVM = s.query.NewValueEditorBuilder(&s.cellEnteredAdd).Build()
	s.arrivedVM = s.query.NewValueEditorBuilder(&s.cellEnteredAdd).
		Remove(goke.Remove[MoveOrder]()).Build()

	s.enteredQuery = si.NewQueryBuilder(&s.cellEnteredClear).Build()
	s.clearEditor = s.enteredQuery.NewEditorBuilder().Remove(goke.Remove[CellEntered]()).Build()
}

func (s *navigationSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	s.clearEnteredTags(cb)
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
		snap := s.query.ChunkSnapshot()

		var enteredIDs []uid.UID64
		var enteredVals []CellEntered
		var arrivedIDs []uid.UID64
		var bothIDs []uid.UID64
		var bothVals []CellEntered

		steers := s.steer.Slice(cursor)
		movers := s.mover.Slice(cursor)
		zs := s.z.Slice(cursor)
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

			entered := false
			moveTo := func(c board.CellID) {
				if c == cells[i].ID {
					return
				}
				cells[i].ID = c
				if entered {
					enteredVals[len(enteredVals)-1] = CellEntered{ID: c}
					return
				}
				entered = true
				enteredIDs = append(enteredIDs, id)
				enteredVals = append(enteredVals, CellEntered{ID: c})
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
					arrivedIDs = append(arrivedIDs, id)
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
						arrivedIDs = append(arrivedIDs, id)
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
				reserved, ok := s.reserveLeg(cells[i].ID, waypoint, id, domain)
				if !ok {
					st.RequestSpeed(0)
					p.Length = 0
					continue
				}
				*leg = reserved
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

			if entered {
				n := len(enteredIDs) - 1
				bothIDs = append(bothIDs, enteredIDs[n])
				bothVals = append(bothVals, enteredVals[n])
				enteredIDs, enteredVals = enteredIDs[:n], enteredVals[:n]
			} else {
				arrivedIDs = append(arrivedIDs, id)
			}
		}

		if len(bothIDs) > 0 {
			vals := cb.AddCompValue(s.arrivedVM, &s.cellEnteredAdd, snap, bothIDs)
			copy(vals, bothVals)
		}
		if len(enteredIDs) > 0 {
			vals := cb.AddCompValue(s.enterVM, &s.cellEnteredAdd, snap, enteredIDs)
			copy(vals, enteredVals)
		}
		if len(arrivedIDs) > 0 {
			buf := s.query.BeginMigrate(cb)
			for _, id := range arrivedIDs {
				buf.Add(id)
			}
			buf.Commit(s.arrivedEditor)
		}
	}
}

// giveWay has every unit standing in cursor's chunk, no hand on it, that one on the move struck,
// give way to it as the keeping says.
func (s *navigationSystem) giveWay(cb *goke.CmdBuf, cursor *goke.Cursor) {
	colls := s.coll.Slice(cursor)
	if colls == nil {
		return
	}
	cells, bases, movers, zs, hands := s.cell.Slice(cursor), s.base.Slice(cursor), s.mover.Slice(cursor), s.z.Slice(cursor), s.hand.Slice(cursor)
	for i, id := range cursor.IDs {
		contacts := colls[i].Contacts()
		if len(contacts) == 0 || hands != nil {
			continue
		}
		m := member{id: id, cell: cells[i].ID, from: cells[i].ID, domain: board.DomainAt(movers, i), pos: bases[i].Pos, vel: bases[i].Vel.Delta(), facing: bases[i].Vel.Dir}
		if zs != nil {
			m.z = zs[i]
		}
		if movers != nil {
			m.lift = movers[i].Lift
		}
		if order, ok := s.keep.yield(m, contacts); ok {
			cb.AddOne(id, s.orderID, order)
		}
	}
}

// gather appends every unit as it stands to dst.
func (s *navigationSystem) gather(dst []body) []body {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, orders, movers := s.base.Slice(cursor), s.order.Slice(cursor), s.mover.Slice(cursor)
		for i, id := range cursor.IDs {
			pos := bases[i].Pos
			b := body{id: id, at: board.Center(pos), half: geom.NewVec(pos.Size.X/2, pos.Size.Y/2), vel: bases[i].Vel.Delta(), domain: board.DomainAt(movers, i), moving: orders != nil}
			if orders != nil {
				b.yielding = orders[i].GivingWay
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
	return st.Speed * dt / st.TurnRate
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

// reserveLeg claims every cell a step from→to can touch, or none of them and false.
func (s *navigationSystem) reserveLeg(from, to board.CellID, id uid.UID64, domain board.Domain) (Leg, bool) {
	leg := Leg{From: from, To: to, Active: true}
	if c1, c2, diag := s.grid.DiagonalNeighbors(from, to); diag {
		leg.C1, leg.C2, leg.Diagonal = c1, c2, true
	}
	for _, c := range leg.cells()[1:] {
		if !s.terrain.Kind(c).Admits(domain) || !s.occupancy.CanEnter(c, id, domain) {
			return Leg{}, false
		}
	}
	for _, c := range leg.cells() {
		s.occupancy.Enter(c, id, domain)
	}
	return leg, true
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

func (s *navigationSystem) clearEnteredTags(cb *goke.CmdBuf) {
	s.enteredQuery.All()
	for s.enteredQuery.Next() {
		cursor := s.enteredQuery.Cursor()
		buf := s.enteredQuery.BeginMigrate(cb)
		for _, id := range cursor.IDs {
			buf.Add(id)
		}
		buf.Commit(s.clearEditor)
	}
}
