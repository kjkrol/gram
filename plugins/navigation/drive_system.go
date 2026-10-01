package navigation

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*driveSystem)(nil)

// driveSystem carries out steering.Driven: an entity steered by hand turns — by Turn, or towards Face —
// and walks the way it faces, sprinting where urged, or brakes to a stop and backs away facing as
// it does, never towards a cell its domain may not stand on or the occupancy keeps it out of; one
// that flies, flown by hand, goes along the ground only the run of the way it is steered along,
// the topography climbing it by the rise. A hand
// on it ends any order it had, and without one it brakes. Its Cell and its hold on the occupancy
// follow it cell by cell, as navigationSystem keeps an ordered entity's.
type driveSystem struct {
	nav *navigationSystem

	query  *goke.Query
	cell   goke.Comp[board.At]
	base   goke.Comp[world.Base]
	steer  goke.Comp[steering.Steering]
	course goke.Comp[steering.Course]
	driven goke.Comp[steering.Driven]
	order  goke.OptComp[MoveOrder]
	mover  goke.OptComp[board.Mover]
	states goke.OptComp[tag.Tags[States]]

	orderID, statesID goke.CompID
}

// driveTurn is how far a hand turns an entity a tick: four degrees.
const driveTurn = math.Pi / 45

// driveMargin is how far past its own edge, in world units, a driven entity looks for the ground
// ahead before it walks on.
const driveMargin = 2.0

func (s *driveSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.base, &s.steer, &s.course, &s.driven).Optional(&s.order).Optional(&s.mover).Optional(&s.states).Build()
	s.orderID = si.RegComp[MoveOrder]()
	s.statesID = si.RegComp[tag.Tags[States]]()
}

func (s *driveSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		cells, bases, steers, courses, drivens := s.cell.Slice(cur), s.base.Slice(cur), s.steer.Slice(cur), s.course.Slice(cur), s.driven.Slice(cur)
		orders, movers, states := s.order.Slice(cur), s.mover.Slice(cur), s.states.Slice(cur)
		for i, id := range cur.IDs {
			in, st, base := drivens[i], steering.Helm{Steering: &steers[i], Course: &courses[i]}, &bases[i]
			domain := board.DomainAt(movers, i)
			facing := in.Face.X != 0 || in.Face.Y != 0
			if orders != nil {
				if in.Ahead == 0 && in.Turn == 0 && !facing {
					continue // nobody at the wheel: the order goes on
				}
				if leg := orders[i].Leg; leg.Active {
					s.nav.releaseLeg(leg, id)
					s.nav.occupancy.Enter(cells[i].Cell, id, domain)
				}
				cb.RemoveCompOne(id, s.orderID)
			}
			if s.follow(id, &cells[i], base.Pos, domain) {
				s.nav.enter(states, i, id)
			}

			heading := base.Vel.Dir
			if heading.X == 0 && heading.Y == 0 {
				heading = geom.NewVec(0, 1)
			}
			switch {
			case facing:
				n := math.Hypot(in.Face.X, in.Face.Y)
				heading = geom.NewVec(in.Face.X/n, in.Face.Y/n)
			case in.Turn != 0:
				a := float64(in.Turn) * driveTurn
				sin, cos := math.Sincos(a)
				heading = geom.NewVec(heading.X*cos-heading.Y*sin, heading.X*sin+heading.Y*cos)
			}
			if in.Turn != 0 || in.Ahead != 0 || facing {
				st.Request(heading) // what it faces now, not a heading an order left behind
			}
			level := 1.0 // a flyer steered up or down goes the less along the ground, the more steeply
			if domain&board.Air != 0 {
				_, level = in.Slope()
			}
			m := member{id: id, cell: cells[i].Cell, from: cells[i].Cell, domain: domain, pos: base.Pos, vel: base.Vel.Delta(), facing: base.Vel.Dir}
			switch {
			case in.Ahead > 0 && s.open(m, heading) && in.Sprint:
				st.RequestSprint()
				st.WantSpeed *= level
			case in.Ahead > 0 && s.open(m, heading):
				st.RequestSpeed(st.MaxSpeed * level)
			case in.Ahead > 0:
				st.RequestSpeed(0)
				st.Speed = 0 // stopped on the spot, at the water's edge as when asked to
			case in.Ahead < 0 && st.Speed > 0:
				st.RequestSpeed(0) // braking before it backs away
			case in.Ahead < 0 && s.open(m, geom.NewVec(-heading.X, -heading.Y)):
				st.RequestBack(backing(st.Steering))
			case in.Ahead < 0:
				st.RequestSpeed(0)
				st.Speed = 0 // stopped at the edge behind it
			default:
				st.RequestSpeed(0)
			}
		}
	}
	for _, id := range s.nav.lacking { // no batch here: a move by id at once
		cb.AddOne(id, s.statesID, tag.Tags[States](0).With(Entered))
	}
	s.nav.lacking = s.nav.lacking[:0]
}

// backing is how fast a driven entity backs away: at the speed it sets off at, a quarter of its
// top speed without one.
func backing(st *steering.Steering) float64 {
	if st.V0 > 0 {
		return st.V0
	}
	return st.MaxSpeed / 4
}

// follow moves the entity's Cell, and its hold on the occupancy, to the cell under its centre;
// true when that is another cell.
func (s *driveSystem) follow(id uid.UID64, cell *board.At, pos world.Position, domain board.Domain) bool {
	actual, ok := s.nav.grid.CellAt(board.Center(pos))
	if !ok || actual == cell.Cell {
		return false
	}
	s.nav.occupancy.Leave(cell.Cell, id)
	s.nav.occupancy.Enter(actual, id, domain)
	cell.Cell = actual
	return true
}

// open reports whether the ground just ahead of m, the way it faces, takes it: a cell its domain
// may stand on, and one the keeping lets it on into.
func (s *driveSystem) open(m member, heading geom.Vec) bool {
	reach := max(m.pos.Size.X, m.pos.Size.Y)/2 + driveMargin
	centre := m.centre()
	ahead, ok := s.nav.grid.CellAt(geom.NewVec(centre.X+heading.X*reach, centre.Y+heading.Y*reach))
	if !ok || !s.nav.terrain.Kind(ahead).Admits(m.domain) {
		return false
	}
	return s.nav.keep.mayStep(m, ahead, heading)
}
