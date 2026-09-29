package steering

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Steering turns a requested heading into motion gradually (Reflex, TurnRate) and, with a motion
// profile (MaxSpeed > 0), drives the base speed too: off at V0, then Accel a second towards WantSpeed.
type Steering struct {
	Want     geom.Vec // heading being turned towards; zero means none yet
	Pending  geom.Vec // heading asked for, taking over from Want once Delay runs out
	TurnRate float64  // radians per tick, 0 to swing all the way at once
	Reflex   uint8    // ticks between a request and acting on it
	Delay    uint8    // ticks still to wait

	MaxSpeed  float64 // top base speed, world units a second; zero means no profile
	Sprint    float64 // how many times MaxSpeed a hand may urge it to (RequestSprint); 0 or 1 none
	Accel     float64 // units a second² to speed up; zero changes speed at once
	Brake     float64 // units a second² to slow down; zero brakes at Accel
	V0        float64 // the speed the entity has the instant it sets off from standing
	Speed     float64 // current base speed, written to Velocity.Value each tick; under zero it backs away
	WantSpeed float64 // the speed asked for; under zero backing away (RequestBack)
}

// Request asks the entity to head towards dir, any length; false while an earlier one is pending.
func (s *Steering) Request(dir geom.Vec) bool {
	if s.Delay > 0 {
		return false
	}
	if n := math.Hypot(dir.X, dir.Y); n > 0 {
		dir = geom.NewVec(dir.X/n, dir.Y/n)
	}
	if s.Reflex == 0 {
		s.Want = dir
		return true
	}
	s.Pending = dir
	s.Delay = s.Reflex
	return true
}

// RequestSpeed asks for a base speed, held within zero and MaxSpeed; nothing without a profile.
func (s *Steering) RequestSpeed(v float64) {
	if s.MaxSpeed <= 0 {
		return
	}
	s.WantSpeed = min(max(v, 0), s.MaxSpeed)
}

// RequestSprint asks for Sprint times the top speed — a hand urging the entity on — the top
// speed itself without a Sprint; nothing without a profile.
func (s *Steering) RequestSprint() {
	if s.MaxSpeed <= 0 {
		return
	}
	s.WantSpeed = s.MaxSpeed * max(s.Sprint, 1)
}

// RequestBack asks the entity to back away at speed v, held within zero and MaxSpeed, facing the
// way it faces: moving on, it brakes to a stop first; nothing without a profile.
func (s *Steering) RequestBack(v float64) {
	if s.MaxSpeed <= 0 {
		return
	}
	s.WantSpeed = -min(max(v, 0), s.MaxSpeed)
}

// Braking is the rate the entity slows at: Brake, or Accel without one.
func (s *Steering) Braking() float64 {
	if s.Brake > 0 {
		return s.Brake
	}
	return s.Accel
}

// advance moves Speed towards WantSpeed as the profile allows, over dt seconds: setting off at
// V0, speeding up at Accel — Sprint times faster sprinting over the top speed — slowing at
// Braking; from moving one way to the other it brakes to a stop first.
func (s *Steering) advance(dt float64) {
	sprint := max(s.Sprint, 1)
	want := min(max(s.WantSpeed, -s.MaxSpeed), s.MaxSpeed*sprint)
	accel := s.Accel
	if want > s.MaxSpeed {
		accel *= sprint
	}
	switch {
	case s.Accel <= 0:
		s.Speed = want
	case s.Speed == 0 && want != 0 && s.V0 > 0:
		s.Speed = math.Copysign(min(s.V0, math.Abs(want)), want)
	case s.Speed*want < 0:
		s.Speed = towards(s.Speed, 0, s.Braking()*dt)
	case math.Abs(want) > math.Abs(s.Speed):
		s.Speed = towards(s.Speed, want, accel*dt)
	default:
		s.Speed = towards(s.Speed, want, s.Braking()*dt)
	}
}

// towards is v moved by at most by towards to, not past it.
func towards(v, to, by float64) float64 {
	if v < to {
		return min(v+by, to)
	}
	return max(v-by, to)
}
