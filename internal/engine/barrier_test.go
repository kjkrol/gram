package engine

import (
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
	e.barrier()
	st.Stack().Composition().Show("hud")
	e.Draw(nil)
	if !slices.Equal(drawn, []string{"main"}) {
		t.Errorf("shown after the barrier, the frame drew %v, want the main scene alone", drawn)
	}
	drawn = nil
	e.barrier()
	e.Draw(nil)
	if !slices.Equal(drawn, []string{"main", "hud"}) {
		t.Errorf("past the next barrier, the frame drew %v, want both", drawn)
	}
}
