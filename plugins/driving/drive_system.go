package driving

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*driveSystem)(nil)

// driveSystem carries out steering.Driven: an entity steered by hand turns — by Turn, or towards
// Look or Face — and walks the way it faces, sprinting where urged, or brakes to a stop and backs
// away facing as it does, never towards a cell its domain may not stand on or the keeping keeps it
// out of; one that flies, flown by hand, goes along the ground only the run of the way it is
// steered along, the topography climbing it by the rise. Without a hand it brakes. Its cell and
// its hold on the occupancy follow it cell by cell. A Driving unit is let go once no hand is on it
// and it stands, or something else steers it along a route of its own (Keeping.Ordered).
type driveSystem struct {
	board     *board.Board   // the ground; nil, none
	occupancy cell.Occupancy // who holds which cell; nil with no board
	keeping   Keeping        // who else is in the way; nil, nobody
	*units

	drivenID, placesID goke.CompID
	lacking            []uid.UID64 // the units entering a cell whose chunk had no unit.States
}

// driveTurn is how far a hand turns an entity a tick: four degrees.
const driveTurn = math.Pi / 45

// driveMargin is how far past its own edge, in world units, a driven entity looks for the ground
// ahead before it walks on.
const driveMargin = 2.0

func (s *driveSystem) Init(si *goke.SysInit) {
	s.build(si)
	s.drivenID = si.RegComp[steering.Driven]()
	s.placesID = si.RegComp[tag.Tags[unit.States]]()
}

func (s *driveSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		bases, steers, courses, drivens := s.base.Slice(cur), s.steer.Slice(cur), s.course.Slice(cur), s.driven.Slice(cur)
		if courses == nil || drivens == nil {
			continue // nobody drives it
		}
		ats, movers, states, places := s.at.Slice(cur), s.mover.Slice(cur), s.states.Slice(cur), s.places.Slice(cur)
		for i, id := range cur.IDs {
			in, st, base := drivens[i], steering.Helm{Steering: &steers[i], Course: &courses[i]}, &bases[i]
			domain := unit.DomainAt(movers, i)
			ordered := s.keeping != nil && s.keeping.Ordered(id)
			if states != nil && states[i].Has(Driving) && !in.Steers() && (ordered || st.Speed == 0) {
				states[i] = states[i].Without(Driving) // let go: it stands, or goes on as ordered
				if in == (steering.Driven{}) {
					cb.RemoveCompOne(id, s.drivenID)
				}
				continue
			}
			if ordered && !in.Steers() {
				continue // nobody at the wheel: the order goes on
			}
			if ats != nil && s.follow(id, &ats[i], base.Pos, domain) {
				s.enter(places, i, id)
			}

			face := in.Face
			if in.Look.X != 0 || in.Look.Y != 0 { // where an eye riding in it looks, before the hand's way
				face = in.Look
			}
			facing := face.X != 0 || face.Y != 0
			heading := base.Vel.Dir
			if heading.X == 0 && heading.Y == 0 {
				heading = geom.NewVec(0, 1)
			}
			switch {
			case facing:
				n := math.Hypot(face.X, face.Y)
				heading = geom.NewVec(face.X/n, face.Y/n)
			case in.Turn != 0:
				a := float64(in.Turn) * driveTurn
				sin, cos := math.Sincos(a)
				heading = geom.NewVec(heading.X*cos-heading.Y*sin, heading.X*sin+heading.Y*cos)
			}
			if in.Turn != 0 || in.Ahead != 0 || facing {
				st.Request(heading) // what it faces now, not a heading an order left behind
			}
			level := 1.0 // a flyer steered up or down goes the less along the ground, the more steeply
			if domain&cell.Air != 0 {
				_, level = in.Slope()
			}
			var here cell.ID
			if ats != nil {
				here = ats[i].Cell
			}
			switch {
			case in.Ahead > 0 && s.open(id, base.Pos, here, domain, heading) && in.Sprint:
				st.RequestSprint()
				st.WantSpeed *= level
			case in.Ahead > 0 && s.open(id, base.Pos, here, domain, heading):
				st.RequestSpeed(st.MaxSpeed * level)
			case in.Ahead > 0:
				st.RequestSpeed(0)
				st.Speed = 0 // stopped on the spot, at the water's edge as when asked to
			case in.Ahead < 0 && st.Speed > 0:
				st.RequestSpeed(0) // braking before it backs away
			case in.Ahead < 0 && s.open(id, base.Pos, here, domain, geom.NewVec(-heading.X, -heading.Y)):
				st.RequestBack(backing(st.Steering))
			case in.Ahead < 0:
				st.RequestSpeed(0)
				st.Speed = 0 // stopped at the edge behind it
			default:
				st.RequestSpeed(0)
			}
		}
	}
	for _, id := range s.lacking { // no batch here: a move by id at once
		cb.AddOne(id, s.placesID, tag.Tags[unit.States](0).With(unit.Entered))
	}
	s.lacking = s.lacking[:0]
}

// backing is how fast a driven entity backs away: at the speed it sets off at, a quarter of its
// top speed without one.
func backing(st *steering.Steering) float64 {
	if st.V0 > 0 {
		return st.V0
	}
	return st.MaxSpeed / 4
}

// follow moves the entity's cell, and its hold on the occupancy, to the cell under its centre;
// true when that is another cell. Without a board nothing moves.
func (s *driveSystem) follow(id uid.UID64, at *unit.At, pos world.Position, domain cell.Domain) bool {
	if s.board == nil {
		return false
	}
	actual, ok := s.board.CellAt(pos.Center())
	if !ok || actual == at.Cell {
		return false
	}
	s.occupancy.Leave(at.Cell, id)
	s.occupancy.Enter(actual, id, domain)
	at.Cell = actual
	return true
}

// enter has the unit at row i of places have unit.Entered on for this step; one whose chunk has
// no family gets it once the chunks are done.
func (s *driveSystem) enter(places []tag.Tags[unit.States], i int, id uid.UID64) {
	if places != nil {
		places[i] = places[i].With(unit.Entered)
	} else {
		s.lacking = append(s.lacking, id)
	}
}

// open reports whether the ground just ahead of the unit id, standing at pos in cell at, the way
// heading points, takes it: a cell its domain may stand on, and one the keeping lets it on into.
func (s *driveSystem) open(id uid.UID64, pos world.Position, at cell.ID, domain cell.Domain, heading geom.Vec) bool {
	if s.board == nil {
		return true
	}
	reach := max(pos.Size.X, pos.Size.Y)/2 + driveMargin
	centre := pos.Center()
	ahead, ok := s.board.CellAt(geom.NewVec(centre.X+heading.X*reach, centre.Y+heading.Y*reach))
	if !ok || !s.board.Kind(ahead).Admits(domain) {
		return false
	}
	return s.keeping == nil || s.keeping.MayStep(id, pos, at, ahead, domain, heading)
}
