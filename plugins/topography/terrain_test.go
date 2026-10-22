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
	hazed := p.Read(0).Hazes()
	p.Write().SetHaze(true)
	asked := p.Read(0).Hazes()
	p.Write().SetHaze(false)
	if hazed || !asked || p.Read(0).Hazes() {
		t.Error("the ground is not clear at first, hazed when asked and clear again")
	}
}
