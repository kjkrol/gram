package topography_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

func TestPlugin_CtrlFHazesTheGroundFromAbove(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	bound := ""
	for _, bd := range p.DefaultBindings() {
		if written := players.Written(bd.Trigger); written == "Ctrl+F" {
			bound = written
		}
	}
	if bound == "" {
		t.Error("Ctrl+F is bound to nothing")
	}
	if p.Hazes() || !p.WithHaze(true).Hazes() || p.WithHaze(false).Hazes() {
		t.Error("the ground is not clear at first, hazed when asked and clear again")
	}
}
