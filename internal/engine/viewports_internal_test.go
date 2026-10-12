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

type screenLayer struct{ inits *int }

func (l screenLayer) Init(*goke.SysInit) { *l.inits++ }
func (screenLayer) Draw(*render.Image)   {}

type worldLayer struct{}

func (*worldLayer) Init(*goke.SysInit)                     {}
func (*worldLayer) DrawWorld(*render.Image, camera.Camera) {}

type neither struct{}

func (neither) Init(*goke.SysInit) {}

type source struct{}

func (source) Init(*goke.SysInit)                   {}
func (source) Compose(*render.Frame, camera.Camera) {}

type scene struct {
	name   string
	layers []render.Layer
}

func (s *scene) Name() string                                                      { return s.name }
func (s *scene) Layers() []render.Layer                                            { return s.layers }
func (s *scene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}
func (s *scene) Focusable() bool                                                   { return true }

func TestCheckLayers_RefusesWhatCannotBeDrawn(t *testing.T) {
	inits := 0
	for name, c := range map[string]struct {
		sc   game.Scene
		want string
	}{
		"neither":             {&scene{name: "s", layers: []render.Layer{neither{}}}, "no Renderer"},
		"a source on its own": {&scene{name: "s", layers: []render.Layer{source{}}}, "NewComposer"},
		"a world layer":       {&scene{name: "s", layers: []render.Layer{&worldLayer{}}}, "render.NewFeed"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkLayers(c.sc, c.sc.Layers()); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("checkLayers = %v, want an error mentioning %q", err, c.want)
			}
		})
	}
	ok := &scene{name: "s", layers: []render.Layer{screenLayer{&inits}, screenLayer{&inits}}}
	if err := checkLayers(ok, ok.Layers()); err != nil {
		t.Errorf("a scene of screen layers: %v", err)
	}
}

func TestEnterStage_InitialisesALayerOnceHoweverManyScenesListIt(t *testing.T) {
	inits := 0
	shared := &screenLayer{&inits}
	stage := &stubStage{}
	var err error
	stage.stack, err = game.NewStack(
		&scene{name: "main", layers: []render.Layer{shared}},
		&scene{name: "hud", layers: []render.Layer{shared, shared}},
	)
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(oneStageGame{stage: stage})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if inits != 1 {
		t.Errorf("the shared layer was initialised %d times, want once", inits)
	}
}
