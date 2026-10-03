package field

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

// bandWorld is a row of eight cells of ten with a wall of the height given on the third and
// fourth, the ground level as level says, in a world with heights.
func bandWorld(t *testing.T, height float64, level func(cell.ID) float64) (*Field, func(x uint32) cell.ID) {
	t.Helper()
	grid := grid.DefaultGrids{}.Square(8, 1, 10)
	cells := terrain.New(grid)
	f := New(grid, cells, level)
	f.SetHeights(true)
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	cellAt := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	wall := cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true, Height: height}
	cells.Set(cellAt(3), wall)
	cells.Set(cellAt(4), wall)
	return f, cellAt
}

// solid is what f holds solid across the whole row for a walker spanning band, by cell, with
// the sides it opens.
func solid(f *Field, band collision.Band) map[uint64]collide.Sides {
	out := map[uint64]collide.Sides{}
	f.Solid(world.Layers(cell.Land), band, geom.NewAABBAt(geom.NewVec(0, 0), 80, 10), func(fb collision.FieldBox) bool {
		out[fb.Cell] = fb.Open
		return true
	})
	return out
}

// A wall of a height stands from below up to its top: a walker on the ground meets it, a flyer
// over its top does not, and a wall on a slope stops whoever comes to its foot or stands on the
// slope. A wall of no height stands at every height.
func TestBand_ASolidCellStandsFromBelowUpToItsHeight(t *testing.T) {
	flat := func(cell.ID) float64 { return 0 }
	f, cellAt := bandWorld(t, 10, flat)
	if got := solid(f, collision.Band{Bottom: 0, Top: 20}); len(got) != 2 {
		t.Errorf("a walker on the ground: %d solid cells, want the two of the wall", len(got))
	}
	if got := solid(f, collision.Band{Bottom: 16, Top: 20}); len(got) != 0 {
		t.Errorf("a shot over the low wall: %d solid cells, want none", len(got))
	}
	if got := solid(f, collision.Everywhere); len(got) != 2 {
		t.Errorf("one of no height: %d solid cells, want the two of the wall", len(got))
	}

	raised := func(c cell.ID) float64 {
		if c == cellAt(3) || c == cellAt(4) {
			return 12
		}
		return 0
	}
	f, cellAt = bandWorld(t, 10, raised)
	if got := solid(f, collision.Band{Bottom: 0, Top: 2}); len(got) != 2 {
		t.Errorf("a short walker at the foot of a wall on a slope: %d solid cells, want two", len(got))
	}
	if got := solid(f, collision.Band{Bottom: 12, Top: 34}); len(got) != 2 {
		t.Errorf("a walker on the slope: %d solid cells, want two", len(got))
	}
	if got := solid(f, collision.Band{Bottom: 30, Top: 40}); len(got) != 0 {
		t.Errorf("a flyer over the wall's top at 22: %d solid cells, want none", len(got))
	}

	f, _ = bandWorld(t, 0, flat)
	if got := solid(f, collision.Band{Bottom: 40, Top: 50}); len(got) != 2 {
		t.Errorf("a wall of no height under a flyer: %d solid cells, want the two of the wall", len(got))
	}
	if f.solidBand(cellAt(3), &cell.Kind{Solid: true, Height: 0}) != collision.Everywhere {
		t.Error("a wall of no height stands at every height")
	}
	if got := f.solidBand(cellAt(3), &cell.Kind{Solid: true, Height: 10}); got.Bottom != math.Inf(-1) || got.Top != 10 {
		t.Errorf("a wall of 10 on the flat spans %v, want from below up to 10", got)
	}
}

// A side of a solid cell is open towards a neighbour the entity passes over: a low wall next to
// a high one is open ground to a shot flying over the low one, so the high wall opens that face.
func TestBand_ASideIsOpenTowardsANeighbourUnderTheEntity(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(8, 1, 10)
	cells := terrain.New(grid)
	f := New(grid, cells, func(cell.ID) float64 { return 0 })
	f.SetHeights(true)
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	cellAt := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	cells.Set(cellAt(3), cell.Kind{Name: cell.Named("low"), Cost: 1, Solid: true, Height: 10})
	cells.Set(cellAt(4), cell.Kind{Name: cell.Named("high"), Cost: 1, Solid: true, Height: 30})

	walker := solid(f, collision.Band{Bottom: 0, Top: 20})
	if open := walker[uint64(cellAt(4))]; open&collide.Left != 0 {
		t.Errorf("to a walker the high wall opens %04b towards the low one, want that face closed", open)
	}
	shot := solid(f, collision.Band{Bottom: 16, Top: 20})
	if _, low := shot[uint64(cellAt(3))]; low {
		t.Error("the low wall is solid to a shot flying over it")
	}
	if open := shot[uint64(cellAt(4))]; open&collide.Left == 0 {
		t.Errorf("to the shot the high wall opens %04b, want its face towards the low wall open", open)
	}
}

// A flat world asks no band: every solid cell stands at every height.
func TestBand_AFlatWorldHasEveryCellEverywhere(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(8, 1, 10)
	cells := terrain.New(grid)
	f := New(grid, cells, func(cell.ID) float64 { return 0 })
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	cellAt := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	cells.Set(cellAt(3), cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true})
	if got := solid(f, collision.Band{Bottom: 40, Top: 50}); len(got) != 1 {
		t.Errorf("%d solid cells for a band on the flat, want the wall whatever the band", len(got))
	}
}
