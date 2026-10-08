package topography_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

// H makes the terrain's shadows coarser and fine again; a game's code does the same.
func TestPlugin_HMakesTheShadowsCoarse(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1})
	bound := false
	for _, bd := range p.DefaultBindings() {
		bound = bound || players.Written(bd.Trigger) == "H"
	}
	if !bound {
		t.Error("H is bound to nothing")
	}
	if p.ShadowsCoarse() || !p.WithCoarseShadows(true).ShadowsCoarse() || p.WithCoarseShadows(false).ShadowsCoarse() {
		t.Error("the shadows are not fine at first, coarse when asked and fine again")
	}
}
