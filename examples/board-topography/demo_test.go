package main

import (
	"os"
	"testing"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// The demo sets itself up without a window: its plugins install, its keys bind without a clash,
// its giants spawn within the world's sizes.
func TestDemo_StartsWithoutAWindow(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	if d.a.world == nil || d.a.topography == nil {
		t.Fatal("the demo's world or topography was not made")
	}
}

// The island has two sides: the player at the keyboard and a rival without one, each with walkers
// of its own kind.
func TestDemo_ThePlayerAndARivalOwnWalkersOfTheirOwn(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	s := d.a
	if s.player.ID != 1 || s.rival.ID != 2 {
		t.Errorf("the player is %d, the rival %d; want players 1 and 2", s.player.ID, s.rival.ID)
	}
	if kind.Named[unitRow](s.world.Kinds(), RivalKind).SpriteID() == kind.Named[unitRow](s.world.Kinds(), UnitKind).SpriteID() {
		t.Error("the rival's walkers are drawn as the player's")
	}
}

// The demo draws its frames: every kind of its units has a sprite in the atlas — the crowd on the
// plateau once had none and the first frame panicked. Needs a GPU; skipped without one.
func TestDemo_DrawsAFrame(t *testing.T) {
	needGPU(t)
	e := engine.NewEngine(NewDemo())
	if err := e.Init(); err != nil {
		t.Fatal(err)
	}
	w, h := e.Layout(640, 360)
	screen := render.NewImage(w, h)
	for range 3 {
		if err := e.Update(); err != nil {
			t.Fatal(err)
		}
		screen.Clear()
		e.Draw(screen)
	}
}

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
