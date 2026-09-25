package navigation

import (
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// MaxWaypoints bounds the goals queued behind a MoveOrder's Target.
const MaxWaypoints = 8

// MoveOrder commands an entity to path toward Target, then through each queued goal in turn;
// navigationSystem removes it once the last is reached.
type MoveOrder struct {
	Target    board.CellID
	Waypoints [MaxWaypoints]board.CellID // goals after Target, in order
	Queued    uint8                      // how many of Waypoints are in use
	Path      Path
	Leg       Leg
	Waited    time.Duration
	// Bumped says the entity struck someone since the last tick; Cooldown is how long it then
	// keeps its new route before it would react to a bump again.
	Bumped   bool
	Cooldown time.Duration
}

// Enqueue adds a goal after the last queued one; false when the queue is full.
func (m *MoveOrder) Enqueue(c board.CellID) bool {
	if int(m.Queued) >= MaxWaypoints {
		return false
	}
	m.Waypoints[m.Queued] = c
	m.Queued++
	return true
}

// advance makes the next queued goal the Target and drops the Path; false with nothing queued.
func (m *MoveOrder) advance() bool {
	if m.Queued == 0 {
		return false
	}
	m.Target = m.Waypoints[0]
	copy(m.Waypoints[:], m.Waypoints[1:m.Queued])
	m.Queued--
	m.Path = Path{}
	return true
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

	query *goke.Query
	cell  goke.Comp[board.Cell]
	base  goke.Comp[world.Base]
	steer goke.Comp[world.Steering]
	order goke.OptComp[MoveOrder]
	mover goke.OptComp[board.Mover]
	route []geom.Vec // the centres ahead, unwrapped, reused each entity

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
	return &navigationSystem{grid: grid, terrain: terrain, occupancy: occupancy, pathFinder: pathFinder}
}

// BindSpace attaches the shared spatial index, so arrivals snap to the cell centre.
func (s *navigationSystem) BindSpace(space *aabbworld.Space) { s.space = space }

func (s *navigationSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.base, &s.steer).
		Optional(&s.order).
		Optional(&s.mover).
		Build()
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

	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		orders := s.order.Slice(cursor)
		if orders == nil {
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
		dt := d.Seconds()

		for i, id := range cursor.IDs {
			domain := board.DomainAt(movers, i)
			target := orders[i].Target
			p := &orders[i].Path
			leg := &orders[i].Leg
			st := &steers[i]
			current := cells[i].ID
			actual, ok := s.grid.CellAt(board.Center(bases[i].Pos))
			if !ok {
				actual = current
			}

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

			orders[i].Cooldown = max(orders[i].Cooldown-d, 0)
			if orders[i].Bumped {
				// struck someone: stop here, plan again from where it stands, and hold that route for
				// bumpInterval however many bumps follow
				orders[i].Bumped, orders[i].Cooldown = false, bumpInterval
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

			if !leg.Active && !s.terrain.Kind(cells[i].ID).Admits(domain) {
				// stuck where it may not be — frozen in, say: the order waits for the ground to change
				st.RequestSpeed(0)
				p.Length = 0
				continue
			}

			if !leg.Active && (p.Length == 0 || p.Index >= p.Length) && cells[i].ID != target {
				newPath, found := s.pathFinder.findPath(id, domain, cells[i].ID, target)
				if !found {
					st.RequestSpeed(0)
					orders[i].Waited += d
					if orders[i].Waited < targetWaitTimeout {
						continue
					}
					dest, destPath, ok := s.pathFinder.nearestFree(id, domain, cells[i].ID, target, nil)
					if !ok {
						arrivedIDs = append(arrivedIDs, id)
						continue
					}
					orders[i].Target, target = dest, dest
					newPath = destPath
				}
				orders[i].Waited = 0
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
			want := s.unwrap(have, s.grid.CellCenter(waypoint))

			reach := lookaheadReach(st, dt)
			if waypoint == target && orders[i].Queued > 0 {
				// a goal with more behind it is passed like a waypoint, then the next one is aimed at
				from := cells[i].ID
				if leg.Active {
					from = leg.From
				}
				if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
					s.drive(st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target), reach, st.MaxSpeed)
					continue
				}
				if leg.Active && actual != leg.To {
					// passed by the lookahead, not yet entered: hold the leg, aim at the next goal
					s.route = append(s.route[:0], s.unwrap(have, s.grid.CellCenter(orders[i].Waypoints[0])))
					s.drive(st, bases[i].Vel.Dir, have, s.route, reach, st.MaxSpeed)
					continue
				}
				if leg.Active {
					s.releaseLeg(*leg, id)
					s.occupancy.Enter(leg.To, id, domain)
					moveTo(leg.To)
					*leg = Leg{}
				}
				orders[i].advance()
				next := s.unwrap(have, s.grid.CellCenter(orders[i].Target))
				s.route = append(s.route[:0], next)
				s.drive(st, bases[i].Vel.Dir, have, s.route, reach, st.MaxSpeed)
				continue
			}

			if waypoint != target {
				from := cells[i].ID
				if leg.Active {
					from = leg.From
				}
				if !passed(have, want, s.unwrap(have, s.grid.CellCenter(from)), reach) {
					s.drive(st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target), reach, st.MaxSpeed)
					continue
				}
				if leg.Active && actual != leg.To {
					// passed by the lookahead, not yet entered — a wide turner looks further ahead than
					// half a cell: hold the leg and aim beyond it, or the next tick finds the unit
					// short of its cell and plans again
					s.drive(st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target)[1:], reach, st.MaxSpeed)
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
				s.drive(st, bases[i].Vel.Dir, have, s.ahead(have, want, p, waypoint, target)[1:], reach, st.MaxSpeed)
				continue
			}

			dx, dy := want.X-have.X, want.Y-have.Y
			dist := math.Hypot(dx, dy)
			if dist > arrivalEpsilon {
				dir := geom.NewVec(dx, dy)
				st.Request(dir)
				st.RequestSpeed(approach(st, dist) * turnFactor(bases[i].Vel.Dir, dir))
				continue
			}

			st.RequestSpeed(0)
			st.Speed = 0

			if s.space != nil && (dx != 0 || dy != 0) {
				s.space.Move(&bases[i].Pos.AABB, geom.NewVec(dx, dy))
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

// ahead is the route to steer by: want, then the centres of the steps after waypoint, up to target.
func (s *navigationSystem) ahead(have, want geom.Vec, p *Path, waypoint, target board.CellID) []geom.Vec {
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
		last = s.unwrap(last, s.grid.CellCenter(p.Steps[k]))
		s.route = append(s.route, last)
	}
	return s.route
}

// maxAhead caps how many centres ahead the lookahead point is sought along.
const maxAhead = 8

// drive asks st for the heading to the lookahead point on route and for speed, the less the
// sharper the turn.
func (s *navigationSystem) drive(st *world.Steering, heading, have geom.Vec, route []geom.Vec, reach, speed float64) {
	at := lookahead(have, route, reach)
	if at == have {
		st.RequestSpeed(speed)
		return
	}
	dir := geom.NewVec(at.X-have.X, at.Y-have.Y)
	st.Request(dir)
	st.RequestSpeed(speed * turnFactor(heading, dir))
}

// lookaheadReach is the turning radius at the current speed: how far ahead to look.
func lookaheadReach(st *world.Steering, dt float64) float64 {
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
// braking would leave it at the arrival radius.
func approach(st *world.Steering, dist float64) float64 {
	brake := st.Braking()
	if brake <= 0 {
		return st.MaxSpeed
	}
	v := max(math.Sqrt(2*brake*dist), math.Sqrt(2*brake*arrivalEpsilon))
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
