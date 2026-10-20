package topography_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

func TestPlugin_HazesTheGroundWhenAsked(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	if p.Hazes() || !p.WithHaze(true).Hazes() || p.WithHaze(false).Hazes() {
		t.Error("the ground is not clear at first, hazed when asked and clear again")
	}
}
