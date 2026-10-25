package atmosphere

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// the ground rising eastwards, under the sea's level along the west
type eastwards struct{}

func (eastwards) At(p geom.Vec) float32 { return p.X - 50 }
func (eastwards) Step() float32         { return 1 }

func TestReliefOf_IsTheLowGroundAndTheHighest(t *testing.T) {
	b, _ := board.NewBoard(grid.DefaultGrids{}.Square(10, 4, 32))
	// the cells' middles 16 to 304 across, the ground there -34 to 254 up
	if low, peaks := reliefOf(eastwards{}, b); low != 0 || peaks != 254 {
		t.Errorf("the ground -34 to 254 up, the lowest tenth under the sea: relief from %v to %v, "+
			"want from the sea's 0 to 254", low, peaks)
	}
	if low, peaks := reliefOf(nil, b); low != 0 || peaks != 0 {
		t.Errorf("on a flat map the relief is %v to %v, want none", low, peaks)
	}
}
