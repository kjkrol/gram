package engine

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/render"
)

// a scene whose drawer notes its name as it is drawn
type notedScene struct {
	name  string
	drawn *[]string
}

func (s notedScene) Name() string { return s.name }

func (s notedScene) Drawers() []render.Renderer { return []render.Renderer{s} }

func (notedScene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

func (notedScene) Focusable() bool { return false }

func (s notedScene) Draw(*render.Image) { *s.drawn = append(*s.drawn, s.name) }

func TestEngine_DrawsTheScenesShownAsTheBarrierFoundThem(t *testing.T) {
	var drawn []string
	st := stage.New("meadow").
		Scenes(func() []game.Scene {
			return []game.Scene{notedScene{"main", &drawn}, notedScene{"hud", &drawn}}
		}).
		Update(func(goke.RunCtx, time.Duration) {})
	e := NewEngine(oneStageGame{stage: st})
	if err := e.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	e.current.prepared = true
	e.barrier(100, 100)
	st.Stack().Composition().Show("hud")
	e.Draw(nil)
	if !slices.Equal(drawn, []string{"main"}) {
		t.Errorf("shown after the barrier, the frame drew %v, want the main scene alone", drawn)
	}
	drawn = nil
	e.barrier(100, 100)
	e.Draw(nil)
	if !slices.Equal(drawn, []string{"main", "hud"}) {
		t.Errorf("past the next barrier, the frame drew %v, want both", drawn)
	}
}

// a scene whose drawer notes being laid and drawn
type layingScene struct{ notedScene }

func (s layingScene) Drawers() []render.Renderer { return []render.Renderer{s} }

func (s layingScene) Lay(w, h int) {
	*s.drawn = append(*s.drawn, fmt.Sprintf("%s laid %dx%d", s.name, w, h))
}

func TestEngine_LaysTheScenesShownOnTheBarrierBeforeTheyAreDrawn(t *testing.T) {
	var drawn []string
	st := stage.New("meadow").
		Scenes(func() []game.Scene {
			return []game.Scene{layingScene{notedScene{"main", &drawn}},
				layingScene{notedScene{"hud", &drawn}}}
		}).
		Update(func(goke.RunCtx, time.Duration) {})
	e := NewEngine(oneStageGame{stage: st})
	if err := e.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	e.current.prepared = true
	e.barrier(320, 200)
	e.Draw(nil)
	if want := []string{"main laid 320x200", "main"}; !slices.Equal(drawn, want) {
		t.Errorf("a frame went %v, want %v: the shown scene laid on the barrier, then drawn, the "+
			"hidden one neither", drawn, want)
	}
}
