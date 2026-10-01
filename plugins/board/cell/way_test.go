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
	if got := (cell.Crossing{Way: cell.Way{Kind: road, Width: 4}}).Over(earth); !got.Graded {
		t.Error("a graded bridge over earth is not graded")
	}
}

// A crossing lets whoever it admits over the cell at its cost, and leaves the way under it as it
// is: a bridge over a river admits walkers and still the water.
func TestCrossing_OverLetsWhoeverItAdmitsOverTheWay(t *testing.T) {
	under := cell.Way{Kind: river, Width: 8}.Over(earth)
	got := cell.Crossing{Way: cell.Way{Kind: bridge, Width: 6}}.Over(under)
	if !got.Admits(cell.Land) || !got.Admits(cell.Water) || got.Name != earth.Name {
		t.Errorf("a bridge over a river is %+v, want walkers and water both, the ground's name", got)
	}
	if c := got.CostFor(cell.Land); c != 0.5 {
		t.Errorf("a walker pays %v on the bridge, want the bridge's 0.5", c)
	}
	if c := got.CostFor(cell.Water); c != under.CostFor(cell.Water) {
		t.Errorf("the water pays %v under the bridge, want the river's %v", c, under.CostFor(cell.Water))
	}
	if got := (cell.Crossing{}).Over(under); got != under {
		t.Errorf("no crossing over a river is %+v, want the river as it is", got)
	}
}
