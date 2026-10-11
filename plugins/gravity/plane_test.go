package gravity_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/rule"
)

// room is a flat board shut in: a ceiling, a floor and a wall on the west.
var room = []string{
	"####",
	"#...",
	"#...",
	"####",
}

// A body at rest on the floor stays on it, borne every step, never landing again.
func TestPlane_ABodyAtRestStaysBorne(t *testing.T) {
	r := newRig(t, scene{level: room, bodies: []body{{x: 25, y: 29}},
		rules: []rule.Rule{landing()}})
	for range 5 {
		r.tick()
	}
	r.heard.given()
	at := r.of(0).at
	for range 100 {
		r.tick()
		if s := r.of(0); s.airborne || math.Abs(s.at.Y-at.Y) > 0.01 {
			t.Fatalf("at rest: %+v, want it borne where it was, at %v", s, at)
		}
	}
	if got := r.heard.given(); len(got) != 0 {
		t.Errorf("at rest it landed %d times", len(got))
	}
}

// Walking into a wall as it falls, a body slides down it in the air and lands on the floor.
func TestPlane_AWallBearsNothing(t *testing.T) {
	r := newRig(t, scene{level: room, bodies: []body{{x: 11.5, y: 13, facing: west}},
		rules: []rule.Rule{landing()}})
	r.walk(0, 5)
	for range 10 {
		r.tick()
	}
	if s := r.of(0); !s.airborne || s.at.X > 11.01 {
		t.Fatalf("by the wall: %+v, want it in the air against it", s)
	}
	for range 2 * tps {
		r.tick()
	}
	if s := r.of(0); s.airborne || math.Abs(s.at.Y-29) > 0.05 {
		t.Errorf("after the fall: %+v, want it on the floor", s)
	}
	if got := r.heard.given(); len(got) != 1 {
		t.Errorf("%d landings, want one: on the floor", len(got))
	}
}

// A leap across a flat world rises as high as the Body says, against the pull.
func TestPlane_ALeapRisesAgainstThePull(t *testing.T) {
	r := newRig(t, scene{level: room, leap: 3, bodies: []body{{x: 25, y: 29}},
		rules: []rule.Rule{landing()}})
	for range 5 {
		r.tick()
	}
	r.heard.given()
	floor := r.of(0).at.Y
	r.leap(0)
	top := 0.0
	for range 2 * tps {
		r.tick()
		top = max(top, floor-r.of(0).at.Y)
	}
	if math.Abs(top-3) > 0.1 {
		t.Errorf("leapt %.3f high, want 3", top)
	}
	if got := r.heard.given(); len(got) != 1 {
		t.Errorf("%d landings, want one", len(got))
	}
}

// The pull turned up, every body falls up and lands on the ceiling, borne by it.
func TestPlane_PulledUpItLandsOnTheCeiling(t *testing.T) {
	r := newRig(t, scene{level: room, bodies: []body{{x: 25, y: 29}},
		rules: []rule.Rule{landing()}})
	for range 5 {
		r.tick()
	}
	r.heard.given()
	r.setPull(gravity.Pull{Y: -10})
	for range 3 * tps {
		r.tick()
	}
	if s := r.of(0); s.airborne || math.Abs(s.at.Y-11) > 0.05 {
		t.Errorf("pulled up: %+v, want it borne by the ceiling", s)
	}
	if got := r.heard.given(); len(got) != 1 {
		t.Errorf("%d landings, want one: on the ceiling", len(got))
	}
}
