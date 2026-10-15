package engine

import (
	"strings"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
)

type screenDrawer struct{ inits *int }

func (l screenDrawer) Init(*goke.SysInit) { *l.inits++ }
func (screenDrawer) Draw(*render.Image)   {}

type worldPicture struct{}

func (*worldPicture) Init(*goke.SysInit)                     {}
func (*worldPicture) DrawWorld(*render.Image, camera.Camera) {}

type neither struct{}

func (neither) Init(*goke.SysInit) {}

type source struct{}

func (source) Init(*goke.SysInit)                   {}
func (source) Compose(*render.Frame, camera.Camera) {}

type scene struct {
	name    string
	drawers []render.Drawer
}

func (s *scene) Name() string             { return s.name }
func (s *scene) Drawers() []render.Drawer { return s.drawers }
func (s *scene) Focusable() bool          { return true }

func (s *scene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

func TestCheckDrawers_RefusesWhatCannotBeDrawn(t *testing.T) {
	inits := 0
	of := func(d render.Drawer) game.Scene { return &scene{name: "s", drawers: []render.Drawer{d}} }
	for name, c := range map[string]struct {
		sc   game.Scene
		want string
	}{
		"neither":             {of(neither{}), "no Renderer"},
		"a source on its own": {of(source{}), "ctx.Draws"},
		"a world picture":     {of(&worldPicture{}), "render.NewFeed"},
	} {
		t.Run(name, func(t *testing.T) {
			err := checkDrawers(c.sc, c.sc.Drawers())
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("checkDrawers = %v, want an error mentioning %q", err, c.want)
			}
		})
	}
	ok := &scene{name: "s", drawers: []render.Drawer{screenDrawer{&inits}, screenDrawer{&inits}}}
	if err := checkDrawers(ok, ok.Drawers()); err != nil {
		t.Errorf("a scene of screen drawers: %v", err)
	}
}

func TestEnterStage_InitialisesADrawerOnceHoweverManyScenesListIt(t *testing.T) {
	inits := 0
	shared := &screenDrawer{&inits}
	stage := &stubStage{}
	var err error
	stage.stack, err = game.NewStack(
		&scene{name: "main", drawers: []render.Drawer{shared}},
		&scene{name: "hud", drawers: []render.Drawer{shared, shared}},
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
