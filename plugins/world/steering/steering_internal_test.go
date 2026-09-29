package steering

import (
	"math"
	"testing"
)

func TestSteering_WithoutV0SetsOffAndAccelerates(t *testing.T) {
	s := Steering{MaxSpeed: 100, Accel: 60}
	s.RequestSpeed(100)
	s.advance(0.5)
	if s.Speed != 30 {
		t.Errorf("speed after half a second from standing without V0 = %v, want 30 at Accel 60", s.Speed)
	}
}

// Asked to back away while moving on, it brakes to a stop at Braking first, then sets off
// backwards at V0 and speeds up to what was asked; asked on again, it brakes to a stop before
// setting off forwards.
func TestSteering_BacksAwayOnlyOnceStopped(t *testing.T) {
	s := Steering{MaxSpeed: 100, Accel: 40, Brake: 80, V0: 10, Speed: 60}
	s.RequestBack(30)
	if s.WantSpeed != -30 {
		t.Fatalf("RequestBack(30) asked for %v, want -30", s.WantSpeed)
	}
	s.advance(0.5)
	if s.Speed != 20 {
		t.Errorf("half a second braking from 60 = %v, want 20 at Brake 80", s.Speed)
	}
	s.advance(0.5)
	if s.Speed != 0 {
		t.Errorf("braking on from 20 = %v, want a stop, not backing yet", s.Speed)
	}
	s.advance(0.1)
	if s.Speed != -10 {
		t.Errorf("setting off backwards = %v, want -V0", s.Speed)
	}
	s.advance(0.25)
	if s.Speed != -20 {
		t.Errorf("a quarter of a second backing on = %v, want -20 at Accel 40", s.Speed)
	}
	s.RequestSpeed(100)
	s.advance(0.1)
	if s.Speed != -12 {
		t.Errorf("asked on while backing = %v, want braking towards a stop, -12", s.Speed)
	}
	s.advance(1)
	if s.Speed != 0 {
		t.Errorf("braking on = %v, want a stop before walking on", s.Speed)
	}
	s.advance(0.1)
	if s.Speed != 10 {
		t.Errorf("setting off forwards = %v, want V0", s.Speed)
	}
	s.RequestBack(500)
	if s.WantSpeed != -100 {
		t.Errorf("RequestBack(500) asked for %v, want -MaxSpeed", s.WantSpeed)
	}
}

// Urged on, it goes up to Sprint times its top speed, Sprint times as quickly past the top; a
// profile without a Sprint goes no faster than its top; a plain request stays under it.
func TestSteering_SprintsToItsSprintOnlyWhenUrged(t *testing.T) {
	s := Steering{MaxSpeed: 10, Sprint: 4, Accel: 20, Speed: 10}
	s.RequestSprint()
	if s.WantSpeed != 40 {
		t.Fatalf("urged on asks %v, want 40", s.WantSpeed)
	}
	s.advance(0.1)
	if s.Speed != 18 {
		t.Errorf("a tenth of a second past the top = %v, want 18 at 4 times Accel", s.Speed)
	}
	s.RequestSpeed(100)
	if s.WantSpeed != 10 {
		t.Errorf("RequestSpeed(100) asks %v, want the top speed 10", s.WantSpeed)
	}
	plain := Steering{MaxSpeed: 10, Accel: 20, Speed: 10}
	plain.RequestSprint()
	plain.advance(1)
	if plain.WantSpeed != 10 || plain.Speed != 10 {
		t.Errorf("urged on without a Sprint: asks %v, goes %v; want the top speed", plain.WantSpeed, plain.Speed)
	}
}

// Flown, the way steered along parts the speed between the rise and the run, 80° at the steepest;
// not flown, all of it goes along the ground.
func TestDriven_SlopePartsTheSpeedOnlyWhenFlown(t *testing.T) {
	if rise, run := (Driven{Flown: true, Climb: 0.6}).Slope(); math.Abs(rise-0.6) > 1e-12 || math.Abs(run-0.8) > 1e-12 {
		t.Errorf("flown at a rise of 0.6: %v up, %v along; want 0.6 and 0.8", rise, run)
	}
	if rise, _ := (Driven{Flown: true, Climb: -1}).Slope(); rise != -Steepest {
		t.Errorf("flown straight down: %v up, want the steepest dive %v", rise, -Steepest)
	}
	if rise, run := (Driven{Climb: 0.6}).Slope(); rise != 0 || run != 1 {
		t.Errorf("not flown: %v up, %v along; want all along the ground", rise, run)
	}
}
