package board_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

var (
	earth = cell.Kind{Name: cell.Named("earth"), Cost: 1, Allows: cell.Land | cell.Air, Veil: 0.2}
	river = cell.Kind{Name: cell.Named("river"), Cost: 1, Allows: cell.Water | cell.Air}
)
var bridge = cell.Kind{Name: cell.Named("bridge"), Cost: 1, Allows: cell.Land | cell.Air}.Costing(cell.Land, 0.5)

// A step goes along a way where the way, or a crossing, links the two cells either way round.
func TestBoard_AlongFollowsTheLinksOfWaysAndCrossings(t *testing.T) {
	g := grid.DefaultGrids{}.Square(3, 3, 10)
	at := func(x, y uint32) cell.ID { c := g.CellIndex(x, y); return c }
	brd := board.NewBoard(g)
	brd.SetAll(earth)
	east, _ := grid.Link(g, at(1, 1), at(2, 1))
	brd.SetWay(at(1, 1), cell.Way{Kind: river, Width: 4, Links: east})
	south, _ := grid.Link(g, at(1, 1), at(1, 2))
	brd.SetCrossing(at(1, 1), cell.Crossing{Way: cell.Way{Kind: earth, Width: 4, Links: south}})
	for _, c := range []struct {
		from, to cell.ID
		want     bool
	}{
		{at(1, 1), at(2, 1), true}, {at(2, 1), at(1, 1), true}, // the way, and back along it
		{at(1, 1), at(1, 2), true}, {at(1, 2), at(1, 1), true}, // the crossing
		{at(1, 1), at(0, 1), false}, {at(1, 1), at(2, 2), false}, // no link that way
		{at(0, 0), at(2, 2), false}, // not neighbours
	} {
		if got := brd.Along(c.from, c.to); got != c.want {
			t.Errorf("Along(%v, %v) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
