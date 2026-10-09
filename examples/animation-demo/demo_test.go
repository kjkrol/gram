package main

import (
	"testing"

	"github.com/kjkrol/gram/internal/engine"
)

// The demo sets itself up without a window: its plugins install, the bugs spawn on their rounds
// with the gait's frames on the atlas.
func TestDemo_StartsWithoutAWindow(t *testing.T) {
	a := &arena{}
	d := NewDemo(a)
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	if a.world == nil || a.board == nil {
		t.Fatal("the demo's world or board was not made")
	}
}
