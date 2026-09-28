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
	Accel     float64 // units a second² to speed up; zero changes speed at once
	Brake     float64 // units a second² to slow down; zero brakes at Accel
	V0        float64 // the speed the entity has the instant it sets off from standing
	Speed     float64 // current base speed, written to Velocity.Value each tick
	WantSpeed float64 // the speed asked for
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

// Braking is the rate the entity slows at: Brake, or Accel without one.
func (s *Steering) Braking() float64 {
	if s.Brake > 0 {
		return s.Brake
	}
	return s.Accel
}

// advance moves Speed towards WantSpeed as the profile allows, over dt seconds.
func (s *Steering) advance(dt float64) {
	want := min(max(s.WantSpeed, 0), s.MaxSpeed)
	switch {
	case s.Accel <= 0:
		s.Speed = want
	case s.Speed == 0 && want > 0 && s.V0 > 0:
		s.Speed = min(s.V0, want)
	case want > s.Speed:
		s.Speed = min(s.Speed+s.Accel*dt, want)
	case want < s.Speed:
		s.Speed = max(s.Speed-s.Braking()*dt, want)
	}
}
