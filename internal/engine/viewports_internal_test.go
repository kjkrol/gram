package engine

import (
	"image"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
)

// screenLayer and worldLayer are layers that only count their Init.
type screenLayer struct{ inits *int }

func (l screenLayer) Init(*goke.SysInit)                   { *l.inits++ }
func (screenLayer) Draw(*ebiten.Image)                     {}
func (l *worldLayer) Init(*goke.SysInit)                   { l.inits++ }
func (*worldLayer) DrawWorld(*ebiten.Image, camera.Camera) {}

type worldLayer struct {
	name  string
	inits int
}

// both is a layer that is a Renderer and a WorldRenderer at once.
type both struct{ worldLayer }

func (both) Draw(*ebiten.Image) {}

// neither is a layer that draws nothing at all.
type neither struct{}

func (neither) Init(*goke.SysInit) {}

// scene is a Scene over layers; viewerScene is one with viewports.
type scene struct {
	name   string
	layers []render.Layer
}

func (s *scene) Name() string                                                      { return s.name }
func (s *scene) Layers() []render.Layer                                            { return s.layers }
func (s *scene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}
func (s *scene) Focusable() bool                                                   { return true }

type viewerScene struct {
	scene
	viewports []render.Viewport
}

func (s *viewerScene) Viewports(image.Rectangle) []render.Viewport { return s.viewports }

func TestPasses_DrawEachRunOfWorldLayersThroughEveryViewportInOrder(t *testing.T) {
	inits := 0
	a, d := screenLayer{&inits}, screenLayer{&inits}
	b, c, e := &worldLayer{name: "b"}, &worldLayer{name: "c"}, &worldLayer{name: "e"}
	left := render.Viewport{Area: image.Rect(0, 0, 50, 100)}
	right := render.Viewport{Area: image.Rect(50, 0, 100, 100)}
	asked := 0
	got := passes(nil, []render.Layer{a, b, c, d, e}, func() []render.Viewport {
		asked++
		return []render.Viewport{left, right}
	})

	describe := func(p pass) string {
		if p.screen != nil {
			return "screen"
		}
		var names []string
		for _, l := range p.world {
			names = append(names, l.(*worldLayer).name)
		}
		return strings.Join(names, "+") + "@" + string(rune('0'+p.index))
	}
	var order []string
	for _, p := range got {
		order = append(order, describe(p))
	}
	want := []string{"screen", "b+c@0", "b+c@1", "screen", "e@0", "e@1"}
	if strings.Join(order, " ") != strings.Join(want, " ") {
		t.Errorf("passes %v, want %v", order, want)
	}
	if got[2].viewport != right {
		t.Errorf("the second pass of b+c goes through %v, want the right half", got[2].viewport.Area)
	}
	if asked == 0 {
		t.Error("the viewports were never asked for")
	}
}

func TestPasses_AScreenOnlySceneNeverAsksForViewports(t *testing.T) {
	inits := 0
	got := passes(nil, []render.Layer{screenLayer{&inits}, screenLayer{&inits}}, func() []render.Viewport {
		t.Fatal("a scene without world layers was asked for viewports")
		return nil
	})
	if len(got) != 2 {
		t.Errorf("%d passes, want the two screen layers", len(got))
	}
}

func TestCheckLayers_RefusesWhatCannotBeDrawn(t *testing.T) {
	inits := 0
	for name, c := range map[string]struct {
		sc   game.Scene
		want string
	}{
		"neither":             {&scene{name: "s", layers: []render.Layer{neither{}}}, "neither"},
		"both":                {&scene{name: "s", layers: []render.Layer{&both{}}}, "both"},
		"world, no viewports": {&scene{name: "s", layers: []render.Layer{&worldLayer{}}}, "Viewports"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkLayers(c.sc, c.sc.Layers()); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("checkLayers = %v, want an error mentioning %q", err, c.want)
			}
		})
	}
	ok := &viewerScene{scene: scene{name: "s", layers: []render.Layer{screenLayer{&inits}, &worldLayer{}}}}
	if err := checkLayers(ok, ok.Layers()); err != nil {
		t.Errorf("a viewer with screen and world layers: %v", err)
	}
}

func TestEnterStage_InitialisesALayerOnceHoweverManyScenesListIt(t *testing.T) {
	shared := &worldLayer{}
	stage := &stubStage{}
	var err error
	stage.stack, err = game.NewStack(
		&viewerScene{scene: scene{name: "main", layers: []render.Layer{shared}}},
		&viewerScene{scene: scene{name: "minimap", layers: []render.Layer{shared, shared}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(oneStageGame{stage: stage})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if shared.inits != 1 {
		t.Errorf("the shared layer was initialised %d times, want once", shared.inits)
	}
}
