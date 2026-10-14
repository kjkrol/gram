package driving

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*driveSystem)(nil)

type driveSystem struct {
	board     *board.Board
	surfaces  func() ground.Surfaces
	occupancy cell.Occupancy
	keeping   Keeping
	*units

	drivenID, placesID goke.CompID
	lacking            []uid.UID64
}

// four degrees a tick
const driveTurn = math.Pi / 45

// world units past the entity's edge
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
			continue
		}
		ats, movers, states, places := s.at.Slice(cur), s.mover.Slice(cur), s.states.Slice(cur),
			s.places.Slice(cur)
		zs, falls := s.z.Slice(cur), s.drift.Present(cur)
		for i, id := range cur.IDs {
			in, st, base := drivens[i], steering.Helm{Steering: &steers[i], Course: &courses[i]}, &bases[i]
			domain := unit.DomainAt(movers, i)
			ordered := s.keeping != nil && s.keeping.Ordered(id)
			// held still by a wall (collision's Slow), the course's speed is no momentum
			if !in.Steers() && base.Vel.Value == 0 && base.Vel.Side == 0 {
				st.Speed = 0
			}
			if states != nil && states[i].Has(Driving) && !in.Steers() && (ordered || st.Speed == 0) {
				states[i] = states[i].Without(Driving)
				st.RequestSide(0)
				st.RequestSpeed(0)
				if in == (steering.Driven{}) {
					cb.RemoveCompOne(id, s.drivenID)
				}
				continue
			}
			if ordered && !in.Steers() {
				st.RequestSide(0)
				continue // nobody at the wheel: the order goes on
			}
			var z *world.Z
			if zs != nil {
				z = &zs[i]
			}
			if ats != nil && s.follow(id, &ats[i], base.Pos, z, domain) {
				s.enter(places, i, id)
			}

			face := in.Face
			// where an eye riding in it looks, before the hand's way
			if in.Look.X != 0 || in.Look.Y != 0 {
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
			// a flyer steered up or down goes the less along the ground, the more steeply
			level := 1.0
			if domain&cell.Air != 0 {
				_, level = in.Slope()
			}
			w := walker{id: id, pos: base.Pos, domain: domain, falls: falls}
			if ats != nil {
				w.at = ats[i].Cell
			}
			if zs != nil {
				w.z = &zs[i]
			}
			right := geom.NewVec(-heading.Y, heading.X)
			switch {
			case in.Side != 0 && s.open(w, geom.NewVec(right.X*float64(in.Side), right.Y*float64(in.Side))):
				side := st.MaxSpeed * level
				if in.Sprint {
					side *= max(st.Sprint, 1)
				}
				st.RequestSide(float64(in.Side) * side)
			default:
				st.RequestSide(0) // at the water's edge beside it too
			}
			// asked to go far off its way, it brakes and turns first
			turning := facing && st.Speed > 0 &&
				heading.X*base.Vel.Dir.X+heading.Y*base.Vel.Dir.Y < turnBrake
			switch {
			case in.Ahead > 0 && turning:
				st.RequestSpeed(0)
			case in.Ahead > 0 && s.open(w, heading) && in.Sprint:
				st.RequestSprint()
				st.WantSpeed *= level
			case in.Ahead > 0 && s.open(w, heading):
				st.RequestSpeed(st.MaxSpeed * level)
			case in.Ahead > 0:
				st.RequestSpeed(0)
				st.Speed = 0 // on the spot, at the water's edge
			case in.Ahead < 0 && st.Speed > 0:
				st.RequestSpeed(0) // braking before it backs away
			case in.Ahead < 0 && s.open(w, geom.NewVec(-heading.X, -heading.Y)):
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

// the cosine of the least turn a walking unit brakes for: 45 degrees off its way
const turnBrake = 0.7

func backing(st *steering.Steering) float64 {
	if st.V0 > 0 {
		return st.V0
	}
	return st.MaxSpeed / 4
}

func (s *driveSystem) follow(id uid.UID64, at *unit.At, pos world.Position, z *world.Z,
	domain cell.Domain) bool {
	if s.board == nil {
		return false
	}
	feet, reach := math.Inf(-1), 0.0
	if z != nil {
		feet, reach = z.Altitude, z.Reach()
	}
	actual, deck, ok := s.board.LevelFor(pos.Center(), feet, reach)
	if !ok || actual == at.Cell {
		return false
	}
	s.occupancy.Leave(at.Cell, id)
	s.occupancy.Enter(actual, deck, id, domain)
	at.Cell = actual
	return true
}

func (s *driveSystem) enter(places []tag.Tags[unit.States], i int, id uid.UID64) {
	if places != nil {
		places[i] = places[i].With(unit.Entered)
	} else {
		s.lacking = append(s.lacking, id)
	}
}

type walker struct {
	id     uid.UID64
	pos    world.Position
	at     cell.ID
	domain cell.Domain
	z      *world.Z
	falls  bool
}

func (s *driveSystem) open(w walker, heading geom.Vec) bool {
	if s.board == nil {
		return true
	}
	reach := max(w.pos.Size.X, w.pos.Size.Y)/2 + driveMargin
	centre := w.pos.Center()
	point := geom.NewVec(centre.X+heading.X*reach, centre.Y+heading.Y*reach)
	ahead, ok := s.board.CellAt(point)
	switch {
	case !ok:
		return false
	case w.falls && w.z != nil && s.surfaces != nil:
		if sf := s.surfaces(); sf != nil {
			floor, _ := sf.Surface(point, w.domain, w.z.Altitude, w.z.Reach())
			if floor > w.z.Altitude+w.z.Reach() {
				return false
			}
		}
	case w.falls:
	case w.z != nil && !s.board.KindFor(ahead, point, w.z.Altitude, w.z.Reach()).Admits(w.domain):
		return false
	case w.z == nil && !s.board.Kind(ahead).Admits(w.domain):
		return false
	}
	if s.keeping == nil {
		return true
	}
	deck := false
	if w.z != nil {
		ahead, deck, _ = s.board.LevelFor(point, w.z.Altitude, w.z.Reach())
	}
	return s.keeping.MayStep(w.id, w.pos, w.at, ahead, deck, w.domain, heading)
}
