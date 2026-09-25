package engine_test

import (
	"testing"

	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
)

func TestLayout_AResizableScreenIsTheWindowAndTheCameraFollowsIt(t *testing.T) {
	stage := &saveLoadTestGame{}
	eng := engine.NewEngine(oneStageGame{stage: stage, props: game.Props{ScreenWidth: 400, ScreenHeight: 300, Resizable: true}})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if w, h := eng.Layout(1200, 900); w != 1200 || h != 900 {
		t.Errorf("Layout(1200, 900) = %d x %d, want the window", w, h)
	}
	if w, h := eng.Camera().Viewport(); w != 1200 || h != 900 {
		t.Errorf("the camera draws to %v x %v, want the window's 1200 x 900", w, h)
	}
	// the world is 1000 wide: at 1200 pixels it is scaled up to cover the window
	if z := eng.Camera().Zoom(); z < 1.2 {
		t.Errorf("zoom %v, want at least 1.2 so the 1000-unit world covers the 1200-pixel window", z)
	}
}

func TestLayout_AFixedScreenStaysWhateverTheWindow(t *testing.T) {
	stage := &saveLoadTestGame{}
	eng := engine.NewEngine(oneStageGame{stage: stage, props: game.Props{ScreenWidth: 400, ScreenHeight: 300}})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if w, h := eng.Layout(1200, 900); w != 400 || h != 300 {
		t.Errorf("Layout(1200, 900) = %d x %d, want the fixed 400 x 300", w, h)
	}
	if w, h := eng.Camera().Viewport(); w != 400 || h != 300 {
		t.Errorf("the camera draws to %v x %v, want the fixed screen", w, h)
	}
}
