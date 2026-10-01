package field

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/world"
)

// The field reads every cell's cover once for all (Ready) and anew when a cell changes.
func TestField_ReadyReadsTheCoverOnceAndAnewWhenACellChanges(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(8, 1, 10)
	cells := terrain.New(grid)
	f := New(grid, cells, func(cell.ID) float64 { return 0 })
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	forest := cell.Kind{Cost: 1, Allows: cell.Land, Veil: 0.5, Veils: cell.Land}
	c3, _ := grid.CellIndex(3, 0)
	c6, _ := grid.CellIndex(6, 0)
	cells.Set(c3, forest)
	walk := func() (stretches [][2]float64) {
		f.Walk(geom.NewVec(0, 5), geom.NewVec(1, 0), 80, world.Layers(cell.Land), func(near, far, _, _, tau float64) bool {
			stretches = append(stretches, [2]float64{near, far})
			return true
		})
		return stretches
	}
	f.Ready()
	if got := walk(); len(got) != 1 || got[0] != [2]float64{30, 40} {
		t.Fatalf("walked through %v, want the forest cell from 30 to 40", got)
	}
	cells.Set(c6, forest)
	if got := walk(); len(got) != 2 || got[1] != [2]float64{60, 70} {
		t.Errorf("walked through %v after a second forest, want it from 60 to 70 too", got)
	}
}

// water is ground a walker does not stand on and nothing stops: not solid.
var water = cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water}

// Overhang is the area of a box over ground that does not take an entity on the layers given — the
// land to a swimmer too — off the board none.
func TestGround_OverhangIsTheAreaOverGroundThatDoesNotTakeTheEntity(t *testing.T) {
	const size = 32
	grid := grid.DefaultGrids{}.Square(4, 4, size)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	cells := terrain.New(grid)
	f := New(grid, cells, func(cell.ID) float64 { return 0 })
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	cells.Set(cellAt(2, 1), water)
	edge := float64(2 * size)
	box := geom.NewAABBAt(geom.NewVec(edge-6, size+2), 10, 10) // 4 of its 10 across over the water
	for _, c := range []struct {
		layers world.Layers
		want   float64
	}{{world.Layers(cell.Land), 40}, {world.Layers(cell.Water), 60}, {world.Layers(cell.Land | cell.Water), 0}, {0, 0}} { // no layers: every plane
		if got := f.Overhang(c.layers, box); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("layers %v: overhang %v, want %v", c.layers, got, c.want)
		}
	}
	if got := f.Overhang(world.Layers(cell.Land), geom.NewAABBAt(geom.NewVec(-5, -5), 10, 10)); got != 0 {
		t.Errorf("off the board the overhang is %v, want none", got)
	}
}
