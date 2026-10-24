package engine

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
)

// countingScene draws the ticks it was last handed: counted by the simulation, published by it,
// swapped in on the barrier and drawn by the frame; its input notes the count drawn next, and
// asks for ask on the input numbered at
type countingScene struct {
	ticks       int
	back, front int
	drawn, seen []int
	at          int
	ask         func(game.Runtime)
}

func (*countingScene) Name() string { return "main" }

func (s *countingScene) Drawers() []render.Renderer { return []render.Renderer{s} }

func (s *countingScene) HandleEvents(_ *control.InputEvents, rt game.Runtime, _ game.Composition) {
	s.seen = append(s.seen, s.front)
	if s.ask != nil && len(s.seen) == s.at {
		s.ask(rt)
	}
}

func (*countingScene) Focusable() bool { return true }

func (s *countingScene) Publish() { s.back = s.ticks }

func (s *countingScene) Swap() { s.front = s.back }

func (s *countingScene) Draw(*render.Image) { s.drawn = append(s.drawn, s.front) }

// countingEngine runs sc's stage a tick a frame, readied
func countingEngine(t *testing.T, sc *countingScene) *Engine {
	t.Helper()
	st := stage.New("meadow").
		Scenes(func() []game.Scene { return []game.Scene{sc} }).
		Update(func(goke.RunCtx, time.Duration) { sc.ticks++ })
	e := NewEngine(oneStageGame{stage: st})
	if err := e.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	e.current.prepared, e.shot = true, &shot{}
	return e
}

// frames advances e n frames and draws each, the last simulation joined
func frames(t *testing.T, e *Engine, n int) error {
	t.Helper()
	for range n {
		var times plugin.FrameTimes
		if _, _, err := e.advance(100, 100, &times); err != nil {
			e.join()
			return err
		}
		e.Draw(nil)
	}
	e.join()
	return nil
}

func TestEngine_TicksTheNextFrameWhileAFrameIsDrawn(t *testing.T) {
	sc := &countingScene{}
	e := countingEngine(t, sc)
	if err := frames(t, e, 3); err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 2, 3}; !slices.Equal(sc.drawn, want) {
		t.Errorf("the frames drew %v ticks, want %v: each what was published before its barrier",
			sc.drawn, want)
	}
	if want := []int{0, 2, 3}; !slices.Equal(sc.seen, want) {
		t.Errorf("the input saw %v ticks drawn next, want %v: the first frame's before its ticks, "+
			"the others' on the barrier", sc.seen, want)
	}
	if sc.ticks != 4 {
		t.Errorf("%d ticks once joined, want 4: the fourth frame's ticked while the third was drawn",
			sc.ticks)
	}
}

func TestEngine_TicksEveryFrameBeforeDrawingItWhenSerial(t *testing.T) {
	sc := &countingScene{}
	e := countingEngine(t, sc)
	e.serial = true
	for range 3 {
		var times plugin.FrameTimes
		if _, _, err := e.advance(100, 100, &times); err != nil {
			t.Fatal(err)
		}
		if e.running {
			t.Fatal("serial, a simulation was left running past the barrier")
		}
		e.Draw(nil)
	}
	if want := []int{1, 2, 3}; !slices.Equal(sc.drawn, want) || sc.ticks != 3 {
		t.Errorf("serial, the frames drew %v ticks of %d, want %v of 3", sc.drawn, sc.ticks, want)
	}
	if want := []int{0, 1, 2}; !slices.Equal(sc.seen, want) {
		t.Errorf("serial, the input saw %v ticks, want %v: each frame's before its ticks", sc.seen,
			want)
	}
}

func TestEngine_RunsNoSimulationPastABarrierThatQuitsOrSwitches(t *testing.T) {
	for name, ask := range map[string]func(game.Runtime){
		"quit":   func(rt game.Runtime) { rt.Quit() },
		"switch": func(rt game.Runtime) { _ = rt.SwitchStage("meadow") },
	} {
		t.Run(name, func(t *testing.T) {
			sc := &countingScene{at: 2, ask: ask}
			e := countingEngine(t, sc)
			var times plugin.FrameTimes
			for range 2 {
				if _, _, err := e.advance(100, 100, &times); err != nil {
					t.Fatal(err)
				}
			}
			if e.running {
				t.Errorf("asked to %s on the barrier, the next frame's simulation runs", name)
			}
			e.join()
		})
	}
}

func TestEngine_EndsAfterTheBarrierQuitAsTheNextFrameTakesItsInput(t *testing.T) {
	sc := &countingScene{at: 2, ask: func(rt game.Runtime) { rt.Quit() }}
	e := countingEngine(t, sc)
	if err := frames(t, e, 3); err != Termination {
		t.Fatalf("quit on the second frame's barrier, the third frame gave %v, want Termination",
			err)
	}
	if e.running || sc.ticks != 2 {
		t.Errorf("ended with a simulation running %v after %d ticks, want none after 2", e.running,
			sc.ticks)
	}
}
