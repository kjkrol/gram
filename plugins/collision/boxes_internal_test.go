package collision

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

func TestBoxes_TheCommandSwitchesTheOutlines(t *testing.T) {
	w := testWorldPlugin()
	p := NewPlugin(w)
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	if p.BoxesShown() || p.Renderer() == nil {
		t.Fatal("the boxes are shown before anyone asked, or there is no layer to draw them")
	}
	for i := range 2 {
		w.Carrier().Put(command.Nobody, Boxes{})
		p.switchBoxes()
		if want := i == 0; p.BoxesShown() != want {
			t.Errorf("Boxes given %d times: shown %v, want %v", i+1, p.BoxesShown(), want)
		}
	}
	var keys []string
	for _, b := range p.DefaultBindings() {
		if b.Type().Name() == "Boxes" {
			keys = append(keys, b.Trigger.(control.KeyPress).Key.String())
		}
	}
	if len(keys) != 1 || keys[0] != "X" {
		t.Errorf("Boxes bound to %v, want Shift+X alone", keys)
	}
}
