package clock_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world/clock"
)

const step = time.Second / 60

// rig is a clock in an ECS whose plan hands the clock one piece of simulation counting its runs.
type rig struct {
	c    *clock.Clock
	ecs  *goke.ECS
	runs int
}

func newRig(t *testing.T, cfg clock.Config) *rig {
	t.Helper()
	r := &rig{c: clock.New(cfg), ecs: goke.New()}
	sys := r.ecs.RegSys(r.c.System())
	r.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(sys, d)
		rc.Sync()
		r.c.Simulate(rc, func(goke.RunCtx, time.Duration) { r.runs++ })
		r.c.Replay(rc, d)
	})
	r.ecs.Setup()
	return r
}

func (r *rig) ticks(n int) int {
	r.runs = 0
	for range n {
		r.ecs.Tick(step)
	}
	return r.runs
}

// At tempo 1 the simulation runs once a tick and game time follows; at 2 and 4 as many times, at ½
// every other tick; in the tactical pause never, and game time stands.
func TestClock_ReplaysTheSimulationAsTheTempoSays(t *testing.T) {
	r := newRig(t, clock.Config{})
	if got := r.ticks(60); got != 60 || r.c.Time() != 60*step {
		t.Fatalf("at tempo 1: %d runs, %v gone by; want 60 and 60 steps", got, r.c.Time())
	}
	for _, c := range []struct {
		tempo float32
		runs  int
	}{{2, 120}, {4, 240}, {0.5, 30}} {
		if err := r.c.SetTempo(c.tempo); err != nil {
			t.Fatal(err)
		}
		before := r.c.Time()
		if got := r.ticks(60); got != c.runs || r.c.Time()-before != time.Duration(c.runs)*step {
			t.Errorf("at tempo %g: %d runs, %v gone by; want %d and %v", c.tempo, got, r.c.Time()-before, c.runs, time.Duration(c.runs)*step)
		}
	}
	r.c.SetPaused(true)
	before := r.c.Time()
	if got := r.ticks(10); got != 0 || r.c.Time() != before {
		t.Errorf("paused: %d runs, time moved by %v; want none", got, r.c.Time()-before)
	}
	if err := r.c.SetTempo(3); err == nil {
		t.Error("a tempo the config does not offer was taken")
	}
}

// The time shown runs on between the ticks with the real time the engine holds toward the next,
// at the tempo, and meets game time at each tick: at ½ the half step the tempo filled counts; in
// the tactical pause it is game time.
func TestClock_TheTimeShownRunsOnBetweenTheTicks(t *testing.T) {
	r := newRig(t, clock.Config{})
	r.ticks(1)
	r.c.Pending(step / 2)
	if got, want := r.c.Shown(), r.c.Time()+step/2; got != want {
		t.Errorf("at tempo 1, half a step on: shown %v, want %v", got, want)
	}
	if err := r.c.SetTempo(0.5); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		r.ticks(1)
		r.c.Pending(0)
		after := r.c.Shown()
		r.c.Pending(step)
		before := r.c.Shown() // a step on: where the next tick lands
		r.ticks(1)
		r.c.Pending(0)
		if r.c.Shown() != before || before-after != step/2 {
			t.Errorf("at ½, tick %d: shown %v a tick after, %v then, %v held toward the next; want half a step apart and meeting", i, after, r.c.Shown(), before)
		}
	}
	r.c.SetPaused(true)
	r.c.Pending(step / 2)
	if r.c.Shown() != r.c.Time() {
		t.Errorf("paused: shown %v, want game time %v", r.c.Shown(), r.c.Time())
	}
}

// A BiggerStep clock replays once, over a step as long as the tempo says.
func TestClock_ABiggerStepReplaysOnceOverALongerStep(t *testing.T) {
	r := newRig(t, clock.Config{BiggerStep: true})
	if err := r.c.SetTempo(4); err != nil {
		t.Fatal(err)
	}
	if got := r.ticks(10); got != 10 || r.c.Time() != 40*step {
		t.Errorf("%d runs, %v gone by; want 10 runs each 4 steps long", got, r.c.Time())
	}
}

// The commands pause and set the tempo along the config's list, and the state lives on the
// clock's entity.
func TestClock_CommandsPauseAndShiftTheTempo(t *testing.T) {
	r := newRig(t, clock.Config{})
	issue := func(cmd any) {
		for _, q := range r.c.Queues() {
			if q.Accepts() == reflect.TypeOf(cmd) {
				q.Put(control.Nobody, cmd)
			}
		}
	}
	issue(clock.Faster{})
	r.ticks(1)
	issue(clock.Faster{})
	issue(clock.Faster{}) // past the end: stays at 4
	r.ticks(1)
	if r.c.Tempo() != 4 {
		t.Errorf("tempo %g after three Faster, want 4", r.c.Tempo())
	}
	issue(clock.Pause{})
	r.ticks(1)
	if !r.c.Paused() || r.c.Written() != "00:00 (paused)" {
		t.Errorf("after Pause: paused %v, written %q", r.c.Paused(), r.c.Written())
	}
	issue(clock.Pause{})
	for range 3 {
		issue(clock.Slower{})
	}
	r.ticks(1)
	if r.c.Paused() || r.c.Tempo() != 0.5 {
		t.Errorf("after Pause and three Slower: paused %v, tempo %g; want going at ½", r.c.Paused(), r.c.Tempo())
	}
}

// Falling behind for a while brings a tempo above 1 down a notch, once, and says so.
func TestClock_FallingBehindBringsTheTempoDown(t *testing.T) {
	r := newRig(t, clock.Config{})
	if err := r.c.SetTempo(4); err != nil {
		t.Fatal(err)
	}
	for range 29 {
		r.c.Behind(true)
	}
	if r.c.Tempo() != 4 {
		t.Fatalf("tempo came down after %d frames behind, want 30", 29)
	}
	r.c.Behind(true)
	if r.c.Tempo() != 2 || r.c.Written() != "00:00 (x2, held back)" {
		t.Errorf("tempo %g, written %q; want 2, held back", r.c.Tempo(), r.c.Written())
	}
	r.c.Behind(false)
	if err := r.c.SetTempo(1); err != nil {
		t.Fatal(err)
	}
	for range 40 {
		r.c.Behind(true)
	}
	if r.c.Tempo() != 1 {
		t.Errorf("tempo %g, want 1 left alone: nothing below it to come down to", r.c.Tempo())
	}
}
