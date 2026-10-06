package main

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/render"
)

// The demo sets itself up without a window: its plugins install, the embers spawn on their
// rounds, the Ember material joins the composer's shader.
func TestDemo_StartsWithoutAWindow(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	if d.a.world == nil || d.a.nav == nil {
		t.Fatal("the demo's world or navigation was not made")
	}
	if !strings.Contains(render.ShaderSource(), "fn Ember(") {
		t.Error("the Ember material did not join the composer's shader")
	}
}
