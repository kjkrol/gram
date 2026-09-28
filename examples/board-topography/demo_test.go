package main

import (
	"testing"

	"github.com/kjkrol/gram/internal/engine"
)

// The demo sets itself up without a window: its plugins install, its keys bind without a clash,
// its giants spawn within the world's sizes.
func TestDemo_StartsWithoutAWindow(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	if d.stage.world == nil || d.stage.topography == nil {
		t.Fatal("the demo's world or topography was not made")
	}
}
