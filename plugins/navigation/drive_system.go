package navigation

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*driveSystem)(nil)

// driveSystem carries out world.Driven: an entity steered by hand turns — by Turn, or towards Face —
// and walks the way it
// faces, never towards a cell its domain may not stand on or the occupancy keeps it out of; a hand
// on it ends any order it had, and without one it brakes. Its Cell and its hold on the occupancy
// follow it cell by cell, as navigationSystem keeps an ordered entity's.
type driveSystem struct {
	nav *navigationSystem

	query  *goke.Query
	cell   goke.Comp[board.Cell]
	base   goke.Comp[world.Base]
	steer  goke.Comp[world.Steering]
	driven goke.Comp[world.Driven]
	order  goke.OptComp[MoveOrder]
	mover  goke.OptComp[board.Mover]

	orderID, enteredID goke.CompID
}

// driveTurn is how far a hand turns an entity a tick: four degrees.
const driveTurn = math.Pi / 45

// driveMargin is how far past its own edge, in world units, a driven entity looks for the ground
// ahead before it walks on.
const driveMargin = 2.0

func (s *driveSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.base, &s.steer, &s.driven).Optional(&s.order).Optional(&s.mover).Build()
	s.orderID = si.RegComp[MoveOrder]()
	s.enteredID = si.RegComp[CellEntered]()
}

func (s *driveSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		cells, bases, steers, drivens := s.cell.Slice(cur), s.base.Slice(cur), s.steer.Slice(cur), s.driven.Slice(cur)
		orders, movers := s.order.Slice(cur), s.mover.Slice(cur)
		for i, id := range cur.IDs {
			in, st, base := drivens[i], &steers[i], &bases[i]
			domain := board.DomainAt(movers, i)
			facing := in.Face.X != 0 || in.Face.Y != 0
			if orders != nil {
				if in.Ahead == 0 && in.Turn == 0 && !facing {
					continue // nobody at the wheel: the order goes on
				}
				if leg := orders[i].Leg; leg.Active {
					s.nav.releaseLeg(leg, id)
					s.nav.occupancy.Enter(cells[i].ID, id, domain)
				}
				cb.RemoveCompOne(id, s.orderID)
			}
			s.follow(cb, id, &cells[i], base.Pos, domain)

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
			if in.Ahead > 0 && s.open(member{id: id, cell: cells[i].ID, from: cells[i].ID, domain: domain, pos: base.Pos, vel: base.Vel.Delta(), facing: base.Vel.Dir}, heading) {
				st.RequestSpeed(st.MaxSpeed)
				continue
			}
			st.RequestSpeed(0)
			if in.Ahead != 0 {
				st.Speed = 0 // stopped on the spot, at the water's edge as when asked to
			}
		}
	}
}

// follow moves the entity's Cell, and its hold on the occupancy, to the cell under its centre.
func (s *driveSystem) follow(cb *goke.CmdBuf, id uid.UID64, cell *board.Cell, pos world.Position, domain board.Domain) {
	actual, ok := s.nav.grid.CellAt(board.Center(pos))
	if !ok || actual == cell.ID {
		return
	}
	s.nav.occupancy.Leave(cell.ID, id)
	s.nav.occupancy.Enter(actual, id, domain)
	cell.ID = actual
	cb.AddOne(id, s.enteredID, CellEntered{ID: actual})
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
