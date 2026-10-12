package cell_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
)

var (
	earth = cell.Kind{Name: cell.Named("earth"), Cost: 1, Allows: cell.Land | cell.Air, Veil: 0.2}
	river = cell.Kind{Name: cell.Named("river"), Cost: 1, Allows: cell.Water | cell.Air}
)
var bridge = cell.Kind{Name: cell.Named("bridge"), Cost: 1, Allows: cell.Land | cell.Air}.Costing(cell.Land, 0.5)

// A way decides who may cross its cell and what it costs; the ground keeps the rest.
func TestWay_OverTakesWhoMayCrossAndTheCostFromTheWay(t *testing.T) {
	got := cell.Way{Kind: river, Width: 8}.Over(earth)
	if got.Allows != river.Allows || got.Name != earth.Name || got.Veil != earth.Veil {
		t.Errorf("a river over earth is %+v, want the river's Allows, the rest the earth's", got)
	}
	if got := (cell.Way{Kind: river}).Over(earth); got != earth {
		t.Errorf("a way of no width over earth is %+v, want the earth as it is", got)
	}
	road := cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land, Graded: true}
	if got := (cell.Way{Kind: road, Width: 4}).Over(earth); !got.Graded {
		t.Error("a graded road over earth is not graded")
	}
}
