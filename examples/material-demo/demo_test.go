package main

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/render"
)

// The demo sets itself up without a window: its plugins install, the ward's material joins the
// composer's shader, the darts spawn on their round.
func TestDemo_StartsWithoutAWindow(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	if d.a.world == nil || d.a.board == nil {
		t.Fatal("the demo's world or board was not made")
	}
	if !strings.Contains(render.ShaderSource(), "fn Ward(") {
		t.Error("the Ward material did not join the composer's shader")
	}
}
