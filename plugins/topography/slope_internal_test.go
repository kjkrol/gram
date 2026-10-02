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

// Up a ramp rising 1 in 5 a walker goes at a third of its speed, down it carefully, slower than on
// the flat, and a flyer over it as on the flat: what the Map's Slope tells the board's Moving
// rule.
func TestSlope_SlowsAClimbAndASteepDescent(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 40, Height: 10}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4}, Heights: true})
	grid := grid.DefaultGrids{}.Square(4, 1, 10)
	b := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	p := NewPlugin(w, b, Config{Cell: 10})
	p.Relief().SetHeights(func(q geom.Vec) float64 { return 0.2 * q.X })
	east, west := geom.NewVec(1, 0), geom.NewVec(-1, 0)
	for _, c := range []struct {
		x    float64
		dir  geom.Vec
		d    cell.Domain
		want float64
	}{
		{15, east, cell.Land, 1 + 10*0.2},
		{25, west, cell.Land, 1 - 0.3 + 5*(0.2-0.1)},
		{35, east, cell.Air, 1},
	} {
		if got := p.Slope(geom.NewVec(c.x, 5), c.dir, c.d); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("at x %v going %v in %v: ×%v as long, want ×%v", c.x, c.dir, c.d, got, c.want)
		}
	}
	if got := b.Map().Slope(geom.NewVec(15, 5), east, cell.Land); math.Abs(got-3) > 1e-6 {
		t.Errorf("the board's map prices the climb ×%v, want the topography's ×3", got)
	}
}
