package engine_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type countedSource struct{ inits, composed int }

func (s *countedSource) Init(*goke.SysInit)                   { s.inits++ }
func (s *countedSource) Compose(*render.Frame, camera.Camera) { s.composed++ }

type drawing struct{ src render.Source }

func (*drawing) Name() string                         { return "drawing" }
func (d *drawing) Install(ctx plugin.Installer) error { ctx.Draws(d.src); return nil }
func (*drawing) RunPlan(goke.RunCtx, time.Duration)   {}
func (*drawing) EventHandler() control.EventHandler   { return nil }
func (*drawing) Serializable() plugin.Serializable    { return nil }

func TestWorld_PictureIsWhatTheStagesPluginsDrawReadiedOnceForEveryScene(t *testing.T) {
	src := &countedSource{}
	var w *world.Plugin
	var pictures []render.Picture
	cam := cameratest.NewFromSpace(100, 100, 0)
	st := stage.New("meadow").
		World(testWorldConfig()).
		Plugins(func(made *world.Plugin) {
			w = made
			w.Made().Add(&drawing{src: src})
		}).
		Scenes(func() []game.Scene {
			pictures = append(pictures, w.Picture(), w.Picture())
			return []game.Scene{
				ui.NewScene("main", ui.Image(render.NewFeed(cam, pictures[0]))),
				ui.NewScene("minimap", ui.Image(render.NewFeed(cam, pictures[1]))),
			}
		}).
		Shows("main", "minimap").
		Update(func(goke.RunCtx, time.Duration) {})
	if err := engine.NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if pictures[0] != pictures[1] {
		t.Errorf("the world gave two pictures, want one for the Stage")
	}
	if src.inits != 1 {
		t.Errorf("the source was readied %d times by two scenes, want once", src.inits)
	}
	pictures[0].DrawWorld(nil, cam)
	if src.composed != 1 {
		t.Errorf("the picture composed the plugin's source %d times, want once", src.composed)
	}
}
