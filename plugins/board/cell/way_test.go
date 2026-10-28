package cell_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
)

var (
	earth = cell.Kind{Name: cell.Named("earth"), Pace: 1, Allows: cell.Land | cell.Air, Veil: 0.2}
	river = cell.Kind{Name: cell.Named("river"), Pace: 1, Allows: cell.Water | cell.Air}
)

var bridge = cell.Kind{Name: cell.Named("bridge"), Pace: 1, Allows: cell.Land | cell.Air}.
	Paced(cell.Land, 1)

func TestWay_OverTakesWhoMayCrossAndTheCostFromTheWay(t *testing.T) {
	got := cell.Way{Kind: river, Width: 8}.Over(earth)
	if got.Allows != river.Allows || got.Name != earth.Name || got.Veil != earth.Veil {
		t.Errorf("a river over earth is %+v, want the river's Allows, the rest the earth's", got)
	}
	if got := (cell.Way{Kind: river}).Over(earth); got != earth {
		t.Errorf("a way of no width over earth is %+v, want the earth as it is", got)
	}
	road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land, Graded: true}
	if got := (cell.Way{Kind: road, Width: 4}).Over(earth); !got.Graded {
		t.Error("a graded road over earth is not graded")
	}
}
