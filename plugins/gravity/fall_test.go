package gravity_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
)

var (
	east = geom.NewVec(1, 0)
	west = geom.NewVec(-1, 0)
)

func TestFall_ALeapRisesToItsHeightAndLandsOnce(t *testing.T) {
	r := newRig(t, scene{leap: 3, bodies: []body{{x: 15, y: 15}}, rules: []act.Rule{landing()}})
	r.tick()
	r.leap(0)
	top := 0.0
	for range 2 * tps {
		r.tick()
		s := r.of(0)
		top = max(top, s.z.Altitude)
		if s.z.Altitude > 0 && !s.airborne {
			t.Fatalf("at %.3f up, not in the air", s.z.Altitude)
		}
	}
	if math.Abs(top-3) > 0.01 {
		t.Errorf("leapt %.3f high, want 3", top)
	}
	if s := r.of(0); s.z.Altitude != 0 || s.airborne || s.drift.Z != 0 {
		t.Errorf("after the leap: %+v, want it standing on the ground", s)
	}
	if got := r.heard.given(); len(got) != 1 || got[0].Command.Hard || got[0].Entity != r.ids[0] {
		t.Errorf("landings: %+v, want one soft one", got)
	}
}

func TestFall_OffADecksSideItFallsAndLands(t *testing.T) {
	deck := &land{from: 10, to: 30, top: 10, thick: 1}
	r := newRig(t, scene{land: deck, bodies: []body{
		{x: 28, y: 15, alt: 10, facing: east},
		{x: 12, y: 25, alt: 10, facing: east},
	}, rules: []act.Rule{landing()}})
	r.walk(0, 5)
	r.walk(1, 5)
	fell := false
	for range 3 * tps {
		r.tick()
		fell = fell || r.of(0).airborne
		if s := r.of(1); s.airborne || s.z.Altitude != 10 {
			t.Fatalf("one walking along the deck: %+v, want it on the deck", s)
		}
		if r.of(1).at.X > 26 {
			r.walk(1, 0)
		}
	}
	if s := r.of(0); !fell || s.airborne || s.z.Altitude != 0 {
		t.Errorf("off the deck: fell %v, now %+v, want it fallen to the ground", fell, s)
	}
	if got := r.heard.given(); len(got) != 1 || !got[0].Command.Hard || got[0].Entity != r.ids[0] {
		t.Errorf("landings: %+v, want one hard one of the body off the deck", got)
	}
}

func TestFall_DownASlopeItNeverLeavesTheGround(t *testing.T) {
	slope := &land{ground: func(p geom.Vec) float64 { return 20 - p.X/2 }}
	r := newRig(t, scene{land: slope, bodies: []body{{x: 5, y: 15, alt: 17.5, facing: east}},
		rules: []act.Rule{landing()}})
	r.walk(0, 5)
	for range 5 * tps {
		r.tick()
		s := r.of(0)
		if s.airborne {
			t.Fatalf("at %v in the air going downhill", s.at)
		}
		if want := 20 - s.at.X/2; math.Abs(s.z.Altitude-want) > 0.1 {
			t.Fatalf("at %v %.3f up, want on the ground at %.3f", s.at, s.z.Altitude, want)
		}
	}
	if r.of(0).at.X < 29 {
		t.Errorf("walked to %v, want it far down the slope", r.of(0).at)
	}
	if got := r.heard.given(); len(got) != 0 {
		t.Errorf("landings %+v going downhill, want none", got)
	}
}

func TestFall_AHeadStrikesTheSlabOverIt(t *testing.T) {
	low := &land{from: 10, to: 30, top: 5, thick: 1}
	r := newRig(t, scene{land: low, leap: 3, bodies: []body{{x: 15, y: 15}}})
	r.tick()
	r.leap(0)
	top := 0.0
	for range 2 * tps {
		r.tick()
		top = max(top, r.of(0).z.Top())
	}
	if top > 4+1e-9 {
		t.Errorf("its head rose to %.3f, through a slab whose underside is 4", top)
	}
	if s := r.of(0); s.airborne || s.z.Altitude != 0 {
		t.Errorf("after the leap: %+v, want it back on the ground", s)
	}
}

func TestFall_OneLeapIsOneLeapAtAnyTempo(t *testing.T) {
	for _, c := range []struct {
		name  string
		shift func(r *rig)
	}{
		{"at ½", func(r *rig) { r.w.Clock().Slower() }},
		{"at 2", func(r *rig) { r.w.Clock().Faster() }},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, scene{leap: 3, bodies: []body{{x: 15, y: 15}},
				rules: []act.Rule{landing()}})
			c.shift(r)
			r.tick()
			r.leap(0)
			top := 0.0
			for range 4 * tps {
				r.tick()
				top = max(top, r.of(0).z.Altitude)
			}
			if math.Abs(top-3) > 0.02 {
				t.Errorf("leapt %.3f high, want 3", top)
			}
			if got := r.heard.given(); len(got) != 1 {
				t.Errorf("%d landings, want one", len(got))
			}
		})
	}
}

func TestFall_AwayThrowsItOffFromAnother(t *testing.T) {
	r := newRig(t, scene{bodies: []body{{x: 15, y: 15}, {x: 25, y: 15}}})
	r.tick()
	away := awayFrom(r.ids[0], 5, 1)
	r.w.Carrier().PutFrom(r.ids[1], away)
	r.tick()
	if s := r.of(1); !s.airborne || s.drift.X <= 0 {
		t.Fatalf("thrown: %+v, want it in the air going east, away from the other", s)
	}
	for range 2 * tps {
		r.tick()
	}
	s := r.of(1)
	if s.airborne || s.drift != (world.Drift{}) || s.at.X < 28 {
		t.Errorf("after the throw: %+v, want it landed further east and still", s)
	}
	if o := r.of(0); o.at.X != 15 || o.airborne {
		t.Errorf("the other: %+v, want it where it stood", o)
	}
}

func TestFall_ALeapClearsARailingOnlyHigherThanIt(t *testing.T) {
	for _, c := range []struct {
		leap  float64
		clear bool
	}{{0.5, false}, {2, true}} {
		r := newRig(t, scene{leap: c.leap, fence: true,
			bodies: []body{{x: 14, y: 15, facing: east}}})
		r.walk(0, 5)
		for r.of(0).at.X < 16.2 {
			r.tick()
		}
		r.leap(0)
		for range 2 * tps {
			r.tick()
		}
		if x := r.of(0).at.X; (x > 21) != c.clear {
			t.Errorf("leaping %v high over a railing 1 high: at x %.2f, past it %v, want %v",
				c.leap, x, x > 21, c.clear)
		}
	}
}

func TestFall_InTheAirOverWaterItHasNotFallenIn(t *testing.T) {
	r := newRig(t, scene{leap: 8, water: true, bodies: []body{{x: 14, y: 15, facing: east}},
		rules: []act.Rule{act.Branch[unit.Standing]{
			Name: "drown",
			When: cond.Self.If(unit.Fallen()),
			Then: act.Self.Do(landed{Hard: true}),
		}}})
	r.walk(0, 5)
	for r.of(0).at.X < 19 {
		r.tick()
	}
	r.leap(0)
	r.tick()
	for ; r.of(0).airborne; r.tick() {
		if got := r.heard.given(); len(got) != 0 {
			t.Fatalf("at %v in the air it has fallen in", r.of(0).at)
		}
	}
	if x := r.of(0).at.X; x < 30 {
		t.Fatalf("landed at x %.2f, short of the water's far side", x)
	}
	r.walk(0, 0)
	r.tick()
	if got := r.heard.given(); len(got) != 0 {
		t.Errorf("landed on the far side, it has fallen in")
	}
}
