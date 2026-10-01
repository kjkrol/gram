package steering

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Steering is how an entity may be steered, a knob the steering only reads and an effect may
// Alter: how fast it turns (TurnRate) and answers (Reflex) and, with a motion profile
// (MaxSpeed > 0), how fast it goes: off at V0, then Accel a second towards the speed asked.
// What it is asked and how far it has come is its Course.
type Steering struct {
	TurnRate float64 // radians per tick, 0 to swing all the way at once
	Reflex   uint8   // ticks between a request and acting on it

	MaxSpeed float64 // top base speed, world units a second; zero means no profile
	Sprint   float64 // how many times MaxSpeed a hand may urge it to (RequestSprint); 0 or 1 none
	Accel    float64 // units a second² to speed up; zero changes speed at once
	Brake    float64 // units a second² to slow down; zero brakes at Accel
	V0       float64 // the speed the entity has the instant it sets off from standing
	Halted   bool    // held where it is, whatever it is asked: no speed, its heading kept
}

// Course is the steering's state of an entity, beside its Steering: the heading and speed asked
// and how far it has come to them. The world gives one to every unit; the steering adds it where
// it is missing.
type Course struct {
	Want      geom.Vec // heading being turned towards; zero means none yet
	Pending   geom.Vec // heading asked for, taking over from Want once Delay runs out
	Delay     uint8    // ticks still to wait
	Speed     float64  // current base speed, written to Velocity.Value each tick; under zero it backs away
	WantSpeed float64  // the speed asked for; under zero backing away (RequestBack)
}

// Helm is an entity's Steering and Course together, as whoever steers it holds them: its
// requests read the one and write the other.
type Helm struct {
	*Steering
	*Course
}

// Steerable reports whether the Helm holds both: the zero Helm is of an entity that cannot be
// steered.
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

// RequestSprint asks for Sprint times the top speed — a hand urging the entity on — the top
// speed itself without a Sprint; nothing without a profile.
func (h Helm) RequestSprint() {
	if h.MaxSpeed <= 0 {
		return
	}
	h.WantSpeed = h.MaxSpeed * max(h.Sprint, 1)
}

// RequestBack asks the entity to back away at speed v, held within zero and MaxSpeed, facing the
// way it faces: moving on, it brakes to a stop first; nothing without a profile.
func (h Helm) RequestBack(v float64) {
	if h.MaxSpeed <= 0 {
		return
	}
	h.WantSpeed = -min(max(v, 0), h.MaxSpeed)
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

// towards is v moved by at most by towards to, not past it.
func towards(v, to, by float64) float64 {
	if v < to {
		return min(v+by, to)
	}
	return max(v-by, to)
}
