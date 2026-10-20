package clock_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

const step = time.Second / 60

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

func TestClock_ReplaysTheSimulationAsTheTempoSays(t *testing.T) {
	r := newRig(t, clock.Config{})
	if got := r.ticks(60); got != 60 || r.c.Time() != 60*step {
		t.Fatalf("at tempo 1: %d runs, %v gone by; want 60 and 60 steps", got, r.c.Time())
	}
	for _, c := range []struct {
		tempo float32
		runs  int
	}{{2, 120}, {4, 240}, {0.5, 30}} {
		if err := r.c.SetExactTempo(c.tempo); err != nil {
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
	if err := r.c.SetExactTempo(3); err == nil {
		t.Error("a tempo the config does not offer was taken")
	}
}

func TestClock_TheTimeShownRunsOnBetweenTheTicks(t *testing.T) {
	r := newRig(t, clock.Config{})
	r.ticks(1)
	r.c.Pending(step / 2)
	if got, want := r.c.Shown(), r.c.Time()+step/2; got != want {
		t.Errorf("at tempo 1, half a step on: shown %v, want %v", got, want)
	}
	if err := r.c.SetExactTempo(0.5); err != nil {
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

func TestClock_StepsNumberTheStepsOfTheSimulation(t *testing.T) {
	for _, bigger := range []bool{false, true} {
		c := clock.New(clock.Config{BiggerStep: bigger})
		ecs := goke.New()
		var seen []uint64
		ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
			c.Simulate(rc, func(goke.RunCtx, time.Duration) { seen = append(seen, c.Steps()) })
			c.Replay(rc, d)
		})
		ecs.Setup()
		tick := func(n int) {
			for range n {
				ecs.Tick(step)
			}
		}
		tick(2)
		if err := c.SetExactTempo(4); err != nil {
			t.Fatal(err)
		}
		tick(2)
		c.SetPaused(true)
		tick(3)
		for i, n := range seen {
			if n != uint64(i) {
				t.Fatalf("bigger step %v: the steps are numbered %v, want 0, 1, 2 and on",
					bigger, seen)
			}
		}
		if want := map[bool]int{false: 10, true: 4}[bigger]; len(seen) != want {
			t.Errorf("bigger step %v: %d steps, want %d", bigger, len(seen), want)
		}
	}
}

func TestClock_ABiggerStepReplaysOnceOverALongerStep(t *testing.T) {
	r := newRig(t, clock.Config{BiggerStep: true})
	if err := r.c.SetExactTempo(4); err != nil {
		t.Fatal(err)
	}
	if got := r.ticks(10); got != 10 || r.c.Time() != 40*step {
		t.Errorf("%d runs, %v gone by; want 10 runs each 4 steps long", got, r.c.Time())
	}
}

func TestClock_CommandsPauseAndShiftTheTempo(t *testing.T) {
	r := newRig(t, clock.Config{})
	r.c.Faster()
	r.ticks(1)
	r.c.Faster()
	r.c.Faster() // past the end: stays at 4
	r.ticks(1)
	if r.c.Tempo() != 4 {
		t.Errorf("tempo %g after three Faster, want 4", r.c.Tempo())
	}
	r.c.TogglePause()
	r.ticks(1)
	if !r.c.Paused() || r.c.Written() != "00:00 (paused)" {
		t.Errorf("after Pause: paused %v, written %q", r.c.Paused(), r.c.Written())
	}
	r.c.TogglePause()
	for range 3 {
		r.c.Slower()
	}
	r.ticks(1)
	if r.c.Paused() || r.c.Tempo() != 0.5 {
		t.Errorf("after Pause and three Slower: paused %v, tempo %g; want going at ½", r.c.Paused(), r.c.Tempo())
	}
}

func TestClock_FallingBehindBringsTheTempoDown(t *testing.T) {
	r := newRig(t, clock.Config{})
	if err := r.c.SetExactTempo(4); err != nil {
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
	if err := r.c.SetExactTempo(1); err != nil {
		t.Fatal(err)
	}
	for range 40 {
		r.c.Behind(true)
	}
	if r.c.Tempo() != 1 {
		t.Errorf("tempo %g, want 1 left alone: nothing below it to come down to", r.c.Tempo())
	}
}

func TestClock_SimulateAllocatesNothingForABoundBlock(t *testing.T) {
	c := clock.New(clock.Config{})
	runs := 0
	block := clock.Block(func(goke.RunCtx, time.Duration) { runs++ })
	ecs := goke.New()
	var allocs float64
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		c.Simulate(rc, block)
		c.Replay(rc, d)
		allocs = testing.AllocsPerRun(100, func() {
			c.Simulate(rc, block)
			c.Replay(rc, d)
		})
	})
	ecs.Setup()
	ecs.Tick(step)
	if allocs != 0 {
		t.Errorf("a tick of a block bound once allocates %v times, want none", allocs)
	}
	if runs != 102 {
		t.Errorf("the block ran %d times, want 102: the warm-up, AllocsPerRun's own and 100", runs)
	}
}

func TestClock_OnSyncSettlesAfterEverySyncOfTheSimulationAndAsAReplayEnds(t *testing.T) {
	c := clock.New(clock.Config{})
	// room for every replay below, so that what the test notes allocates nothing
	heard := make([]string, 0, 64)
	c.OnSync(func() { heard = append(heard, "settle") })
	block := clock.Block(func(ctx goke.RunCtx, _ time.Duration) {
		heard = append(heard, "block")
		ctx.Sync()
		ctx.Sync()
	})
	ecs := goke.New()
	var allocs float64
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		c.Simulate(rc, block)
		c.Replay(rc, d)
		allocs = testing.AllocsPerRun(10, func() {
			c.Simulate(rc, block)
			c.Replay(rc, d)
		})
	})
	ecs.Setup()
	ecs.Tick(step)
	if allocs != 0 {
		t.Errorf("a replay settling after its Syncs allocates %v times, want none", allocs)
	}
	if want := []string{"block", "settle", "settle", "settle"}; !slices.Equal(heard[:4], want) {
		t.Errorf("heard %v in a replay, want %v", heard[:4], want)
	}

	c.SetPaused(true)
	heard = heard[:0]
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		c.Simulate(rc, block)
		c.Replay(rc, d)
	})
	ecs.Tick(step)
	if want := []string{"settle"}; !slices.Equal(heard, want) {
		t.Errorf("heard %v in the pause, want %v: what synced before the replay settled", heard,
			want)
	}
}

func TestClock_SettlingSettlesAfterEverySyncOutsideReplayToo(t *testing.T) {
	c := clock.New(clock.Config{})
	heard := make([]string, 0, 64)
	block := clock.Block(func(ctx goke.RunCtx, _ time.Duration) {
		heard = append(heard, "block")
		ctx.Sync()
	})
	ecs := goke.New()
	var allocs float64
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		if got := c.Settling(rc); got != rc {
			t.Errorf("without an OnSync Settling hands %T, want the plan's own ctx", got)
		}
		c.OnSync(func() { heard = append(heard, "settle") })
		ctx := c.Settling(rc)
		if again := c.Settling(ctx); again != ctx {
			t.Error("Settling wraps what already settles")
		}
		heard = append(heard, "update")
		ctx.Sync()
		c.Simulate(ctx, block)
		c.Replay(ctx, d)
		allocs = testing.AllocsPerRun(10, func() { c.Settling(rc).Sync() })
	})
	ecs.Setup()
	ecs.Tick(step)
	want := []string{"update", "settle", "block", "settle", "settle"}
	if !slices.Equal(heard[:5], want) {
		t.Errorf("heard %v, want %v", heard[:5], want)
	}
	if allocs != 0 {
		t.Errorf("a Sync through Settling allocates %v times, want none", allocs)
	}
}
