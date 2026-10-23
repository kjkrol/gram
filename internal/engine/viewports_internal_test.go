package engine

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
)

type screenDrawer struct{ inits *int }

func (l screenDrawer) Init(*goke.SysInit) { *l.inits++ }
func (screenDrawer) Draw(*render.Image)   {}

type scene struct {
	name    string
	drawers []render.Renderer
}

func (s *scene) Name() string               { return s.name }
func (s *scene) Drawers() []render.Renderer { return s.drawers }
func (s *scene) Focusable() bool            { return true }

func (s *scene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

func TestEnterStage_InitialisesADrawerOnceHoweverManyScenesListIt(t *testing.T) {
	inits := 0
	shared := &screenDrawer{&inits}
	stage := &stubStage{}
	var err error
	stage.stack, err = game.NewStack(
		&scene{name: "main", drawers: []render.Renderer{shared}},
		&scene{name: "hud", drawers: []render.Renderer{shared, shared}},
	)
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(oneStageGame{stage: stage})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if inits != 1 {
		t.Errorf("the shared drawer was initialised %d times, want once", inits)
	}
}
