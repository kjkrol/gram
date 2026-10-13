package steering

import (
	"github.com/kjkrol/ceremony/effect"
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Steering is how an entity may be steered, a knob an effect may Alter. MaxSpeed 0 is no motion
// profile: the speed is left alone.
type Steering struct {
	TurnRate float64 // radians a tick; 0 swings all the way at once
	Reflex   uint8   // ticks between a request and acting on it

	MaxSpeed float64 // world units a second
	Sprint   float64 // times MaxSpeed when urged on; 0 or 1 none
	Accel    float64 // units a second²; 0 changes speed at once
	Brake    float64 // units a second²; 0 brakes at Accel
	V0       float64 // the speed on setting off from standing
	Halted   bool    // no speed, the heading kept, whatever is asked
}

// Knob: an effect may turn it (effect.Knob).
func (Steering) Knob() {}

var _ effect.Knob = Steering{}

// Course is the steering's state of an entity: the heading and speed asked and how far it has
// come to them.
type Course struct {
	Want      geom.Vec // zero, none yet
	Pending   geom.Vec // takes over from Want once Delay runs out
	Delay     uint8
	Speed     float64 // under zero, backing away
	WantSpeed float64 // under zero, backing away
	Side      float64 // across the heading, to the right over zero: a strafe, at once
}

// Helm is an entity's Steering and Course together, as whoever steers it holds them.
type Helm struct {
	*Steering
	*Course
}

// Steerable reports whether the Helm holds both.
func (h Helm) Steerable() bool { return h.Steering != nil && h.Course != nil }

// Request asks the entity to head towards dir, any length; false while an earlier one is pending.
func (h Helm) Request(dir geom.Vec) bool {
	if h.Delay > 0 {
		return false
	}
	if n := math.Hypot(dir.X, dir.Y); n > 0 {
		dir = geom.NewVec(dir.X/n, dir.Y/n)
	}
	if h.Reflex == 0 {
		h.Want = dir
		return true
	}
	h.Pending = dir
	h.Delay = h.Reflex
	return true
}

// RequestSpeed asks for a base speed, held within zero and MaxSpeed; nothing without a profile.
func (h Helm) RequestSpeed(v float64) {
	if h.MaxSpeed <= 0 {
		return
	}
	h.WantSpeed = min(max(v, 0), h.MaxSpeed)
}

// RequestSprint asks for Sprint times the top speed, the top speed itself without a Sprint;
// nothing without a profile.
func (h Helm) RequestSprint() {
	if h.MaxSpeed <= 0 {
		return
	}
	h.WantSpeed = h.MaxSpeed * max(h.Sprint, 1)
}

// RequestBack asks the entity to back away at speed v, held within zero and MaxSpeed; nothing
// without a profile.
func (h Helm) RequestBack(v float64) {
	if h.MaxSpeed <= 0 {
		return
	}
	h.WantSpeed = -min(max(v, 0), h.MaxSpeed)
}

// RequestSide asks the entity to strafe at v across its heading, to its right over zero, held
// within the top speed (Sprint times it); nothing without a profile.
func (h Helm) RequestSide(v float64) {
	if h.MaxSpeed <= 0 {
		return
	}
	top := h.MaxSpeed * max(h.Sprint, 1)
	h.Side = min(max(v, -top), top)
}

// Braking is the rate the entity slows at: Brake, or Accel without one.
func (s *Steering) Braking() float64 {
	if s.Brake > 0 {
		return s.Brake
	}
	return s.Accel
}

// from moving one way to the other it brakes to a stop first
func (h Helm) advance(dt float64) {
	sprint := max(h.Sprint, 1)
	want := min(max(h.WantSpeed, -h.MaxSpeed), h.MaxSpeed*sprint)
	accel := h.Accel
	if want > h.MaxSpeed {
		accel *= sprint
	}
	switch {
	case h.Accel <= 0:
		h.Speed = want
	case h.Speed == 0 && want != 0 && h.V0 > 0:
		h.Speed = math.Copysign(min(h.V0, math.Abs(want)), want)
	case h.Speed*want < 0:
		h.Speed = towards(h.Speed, 0, h.Braking()*dt)
	case math.Abs(want) > math.Abs(h.Speed):
		h.Speed = towards(h.Speed, want, accel*dt)
	default:
		h.Speed = towards(h.Speed, want, h.Braking()*dt)
	}
}

func towards(v, to, by float64) float64 {
	if v < to {
		return min(v+by, to)
	}
	return max(v-by, to)
}
