package board

import "testing"

// around tells each cell once, its seed first, ring by ring, on square and hex grids alike.
func TestAround_TellsEachCellOnceRingByRing(t *testing.T) {
	for name, grid := range map[string]Grid{
		"square": DefaultGrids{}.Square(9, 9, 10),
		"hex":    DefaultGrids{}.Hex(9, 9, 5),
	} {
		brd := NewBoard(grid, NewTerrainMap())
		middle, _ := grid.CellIndex(4, 4)
		for rings := range 4 {
			told := map[CellID]int{}
			var order []CellID
			brd.around(func(add func(CellID)) { add(middle) }, rings, func(c CellID) {
				told[c]++
				order = append(order, c)
			})
			if order[0] != middle {
				t.Errorf("%s, %d rings: first told %d, want the seed %d", name, rings, order[0], middle)
			}
			for c, n := range told {
				if n != 1 {
					t.Errorf("%s, %d rings: cell %d told %d times, want once", name, rings, c, n)
				}
			}
			if rings > 0 && len(told) <= 1 {
				t.Errorf("%s, %d rings: told %d cells, want the rings round the seed too", name, rings, len(told))
			}
		}
	}
}
