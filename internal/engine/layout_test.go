package engine_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// viewedStage is a world 1000 wide seen through a camera by its one scene, a ui screen of the
// world's feed over the whole screen.
type viewedStage struct {
	cams  *cameras.Plugin
	cam   camera.Camera
	scene *ui.Scene
	stack game.Scenes
}

func (s *viewedStage) Name() string { return "stage" }
func (s *viewedStage) Init(ctx game.Initializer) error {
	s.cams = cameras.NewPlugin(ctx.UseWorld(testWorldConfig()))
	s.cam = s.cams.New(cameras.TopDown(), camera.Config{})
	if err := ctx.Use(s.cams); err != nil {
		return err
	}
	s.Stack().Composition().Show("view")
	return nil
}
func (s *viewedStage) Restore(game.Persistence) (bool, error) { return false, nil }
func (s *viewedStage) Spawn() error                           { return nil }
func (s *viewedStage) Update(goke.RunCtx, time.Duration)      {}
func (s *viewedStage) Stack() game.Scenes {
	if s.stack == nil {
		s.scene = ui.NewScene("view", ui.Image(render.NewFeed(s.cam, nil)))
		s.stack, _ = game.NewStack(s.scene)
	}
	return s.stack
}

func TestLayout_AResizableScreenIsTheWindowAndTheCameraFollowsIt(t *testing.T) {
	stage := &viewedStage{}
	eng := engine.NewEngine(oneStageGame{stage: stage, props: game.Props{ScreenWidth: 400, ScreenHeight: 300, Resizable: true}})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if w, h := eng.Layout(1200, 900); w != 1200 || h != 900 {
		t.Errorf("Layout(1200, 900) = %d x %d, want the window", w, h)
	}
	stage.scene.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1200, 900))) // as a frame on that screen does
	cam := stage.cam
	if w, h := cam.Viewport(); w != 1200 || h != 900 {
		t.Errorf("the camera draws to %v x %v, want the window's 1200 x 900", w, h)
	}
	// the world is 1000 wide: at 1200 pixels it is scaled up to cover the window
	if z := cam.Zoom(); z < 1.2 {
		t.Errorf("zoom %v, want at least 1.2 so the 1000-unit world covers the 1200-pixel window", z)
	}
}

func TestLayout_AFixedScreenStaysWhateverTheWindow(t *testing.T) {
	stage := &viewedStage{}
	eng := engine.NewEngine(oneStageGame{stage: stage, props: game.Props{ScreenWidth: 400, ScreenHeight: 300}})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if w, h := eng.Layout(1200, 900); w != 400 || h != 300 {
		t.Errorf("Layout(1200, 900) = %d x %d, want the fixed 400 x 300", w, h)
	}
	stage.scene.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 300)))
	if w, h := stage.cam.Viewport(); w != 400 || h != 300 {
		t.Errorf("the camera draws to %v x %v, want the fixed screen", w, h)
	}
}
