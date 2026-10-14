package world_test

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/plugins/world"
)

func TestBoxes_TheCommandSwitchesTheOutlines(t *testing.T) {
	r := newRig(t, false, func(*rig) {})
	if r.w.BoxesShown() {
		t.Fatal("the boxes are shown before anyone asked")
	}
	r.w.Carrier().Put(command.Nobody, world.Boxes{})
	r.fxTick()
	if !r.w.BoxesShown() {
		t.Fatal("Boxes given: the outlines stay hidden")
	}
	r.w.Carrier().Put(command.Nobody, world.Boxes{})
	r.fxTick()
	if r.w.BoxesShown() {
		t.Error("Boxes given again: the outlines stay shown")
	}
	if r.w.BoxesRenderer() == nil {
		t.Error("no layer to put in a composer")
	}
}
