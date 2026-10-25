package gravity_test

import (
	"testing"

	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/gravity"
)

var room = []string{
	"####",
	"#...",
	"#...",
	"####",
}

func TestPlane_ABodyAtRestStaysBorne(t *testing.T) {
	r := newRig(t, scene{level: room, bodies: []body{{x: 25, y: 29}},
		rules: []act.Rule{landing()}})
	for range 5 {
		r.tick()
	}
	r.heard.given()
	at := r.of(0).at
	for range 100 {
		r.tick()
		if s := r.of(0); s.airborne || f32.Abs(s.at.Y-at.Y) > 0.01 {
			t.Fatalf("at rest: %+v, want it borne where it was, at %v", s, at)
		}
	}
	if got := r.heard.given(); len(got) != 0 {
		t.Errorf("at rest it landed %d times", len(got))
	}
}

func TestPlane_AWallBearsNothing(t *testing.T) {
	r := newRig(t, scene{level: room, bodies: []body{{x: 11.5, y: 13, facing: west}},
		rules: []act.Rule{landing()}})
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
	if s := r.of(0); s.airborne || f32.Abs(s.at.Y-29) > 0.05 {
		t.Errorf("after the fall: %+v, want it on the floor", s)
	}
	if got := r.heard.given(); len(got) != 1 {
		t.Errorf("%d landings, want one: on the floor", len(got))
	}
}

func TestPlane_ALeapRisesAgainstThePull(t *testing.T) {
	r := newRig(t, scene{level: room, leap: 3, bodies: []body{{x: 25, y: 29}},
		rules: []act.Rule{landing()}})
	for range 5 {
		r.tick()
	}
	r.heard.given()
	floor := r.of(0).at.Y
	r.leap(0)
	top := float32(0)
	for range 2 * tps {
		r.tick()
		top = max(top, floor-r.of(0).at.Y)
	}
	if f32.Abs(top-3) > 0.1 {
		t.Errorf("leapt %.3f high, want 3", top)
	}
	if got := r.heard.given(); len(got) != 1 {
		t.Errorf("%d landings, want one", len(got))
	}
}

func TestPlane_PulledUpItLandsOnTheCeiling(t *testing.T) {
	r := newRig(t, scene{level: room, bodies: []body{{x: 25, y: 29}},
		rules: []act.Rule{landing()}})
	for range 5 {
		r.tick()
	}
	r.heard.given()
	r.setPull(gravity.Pull{Y: -10})
	for range 3 * tps {
		r.tick()
	}
	if s := r.of(0); s.airborne || f32.Abs(s.at.Y-11) > 0.05 {
		t.Errorf("pulled up: %+v, want it borne by the ceiling", s)
	}
	if got := r.heard.given(); len(got) != 1 {
		t.Errorf("%d landings, want one: on the ceiling", len(got))
	}
}
