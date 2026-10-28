package topography

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

func TestRise_IsRiseOverRunEitherWay(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 40, Height: 10},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4}, Heights: true})
	grid := grid.DefaultGrids{}.Square(4, 1, 10)
	b := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	p := NewPlugin(w, b, nil, Config{})
	p.write.Relief().SetHeights(func(q geom.Vec) float32 { return 0.2 * q.X })
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }
	if got := p.Rise(at(1), at(2)); math.Abs(got-0.2) > 1e-6 {
		t.Errorf("a step east rises %v, want 0.2", got)
	}
	if got := p.Rise(at(2), at(1)); math.Abs(got+0.2) > 1e-6 {
		t.Errorf("a step west rises %v, want -0.2", got)
	}
	if got := b.Read(0).Board().Map().Rise(at(1), at(2)); math.Abs(got-0.2) > 1e-6 {
		t.Errorf("the board's map says a step east rises %v, want the topography's 0.2", got)
	}
}
