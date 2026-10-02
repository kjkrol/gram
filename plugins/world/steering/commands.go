package steering

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// The commands a steered entity gives itself (Order in a rule or a plan), carried out by the
// System as a Request on its Helm. An entity heeds one a step: the first Away it gave, else the
// first Toward, else the first Turn. Away and Toward are told whom they are about as they are
// given (rule.Aimed).
type (
	// Away heads the entity away from From.
	Away struct{ From uid.UID64 }
	// Toward heads the entity at To.
	Toward struct{ To uid.UID64 }
	// Turn heads the entity Angle radians off the way it goes, from +X towards +Y.
	Turn struct{ Angle float64 }
)

// Aim tells the command whom to head away from.
func (a *Away) Aim(who uid.UID64) { a.From = who }

// Aim tells the command whom to head at.
func (t *Toward) Aim(who uid.UID64) { t.To = who }

// queues are where the commands land, for the world to carry.
type queues struct {
	aways   control.Queue[Away]
	towards control.Queue[Toward]
	turns   control.Queue[Turn]
}

// Queues are Away's, Toward's and Turn's; the world carries them.
func (s *System) Queues() []control.CommandQueue {
	return []control.CommandQueue{&s.told.aways, &s.told.towards, &s.told.turns}
}

// obey carries out the commands given since the last step, one for each entity.
func (s *System) obey() {
	clear(s.obeyed)
	s.told.aways.Drain(func(i control.Issued[Away]) {
		if from, ok := s.centre(i.Command.From); ok && i.ByEntity {
			s.head(i.Entity, func(at, _ geom.Vec) geom.Vec { return geom.NewVec(at.X-from.X, at.Y-from.Y) })
		}
	})
	s.told.towards.Drain(func(i control.Issued[Toward]) {
		if to, ok := s.centre(i.Command.To); ok && i.ByEntity {
			s.head(i.Entity, func(at, _ geom.Vec) geom.Vec { return geom.NewVec(to.X-at.X, to.Y-at.Y) })
		}
	})
	s.told.turns.Drain(func(i control.Issued[Turn]) {
		if i.ByEntity {
			sin, cos := math.Sincos(i.Command.Angle)
			s.head(i.Entity, func(_, dir geom.Vec) geom.Vec {
				return geom.NewVec(dir.X*cos-dir.Y*sin, dir.X*sin+dir.Y*cos)
			})
		}
	})
}

// head asks id, steerable and not yet told this step, for the heading way gives from its centre
// and the way it goes; a zero one is no heading.
func (s *System) head(id uid.UID64, way func(at, dir geom.Vec) geom.Vec) {
	if s.obeyed[id] || !s.lookup.Seek(id) {
		return
	}
	cur := s.lookup.Cursor()
	h := Helm{Steering: s.lookSteer.At(cur), Course: s.lookCourse.At(cur)}
	if !h.Steerable() {
		return
	}
	b := s.lookBase.At(cur)
	if dir := way(b.Pos.Center(), b.Vel.Dir); dir.X != 0 || dir.Y != 0 {
		s.obeyed[id] = true
		h.Request(dir)
	}
}

// centre is where id's box stands, its middle; false for an entity gone.
func (s *System) centre(id uid.UID64) (geom.Vec, bool) {
	if !s.lookup.Seek(id) {
		return geom.Vec{}, false
	}
	return s.lookBase.At(s.lookup.Cursor()).Pos.Center(), true
}
