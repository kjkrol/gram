package navigation

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

func TestPathRenderer_PreviewsTheRouteToEachQueuedGoal(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(10, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	r := &pathRenderer{grid: grid, finder: newPathFinder(grid, terrain, nil, &cell.SingleOccupancy{})}

	mt := MoveOrder{Target: at(2)}
	mt.Enqueue(Goal{Cell: at(5)})
	mt.Enqueue(Goal{Cell: at(7)})
	routes := r.queued(uid.UID64(1), cell.Land, &mt)

	if len(routes) != 2 {
		t.Fatalf("got %d routes, want one per queued goal (2)", len(routes))
	}
	want := [][]cell.ID{{at(2), at(3), at(4), at(5)}, {at(5), at(6), at(7)}}
	for k, route := range routes {
		if len(route) != len(want[k]) {
			t.Fatalf("route %d = %v, want %v", k, route, want[k])
		}
		for i := range route {
			if route[i] != want[k][i] {
				t.Fatalf("route %d = %v, want %v", k, route, want[k])
			}
		}
	}

	again := r.queued(uid.UID64(1), cell.Land, &mt)
	if &again[0][0] != &routes[0][0] {
		t.Error("unchanged goals were planned again instead of reusing the kept routes")
	}
	mt.Enqueue(Goal{Cell: at(9)})
	if fresh := r.queued(uid.UID64(1), cell.Land, &mt); len(fresh) != 3 {
		t.Errorf("after another goal got %d routes, want 3", len(fresh))
	}
	mt = MoveOrder{Target: at(2)}
	if left := r.queued(uid.UID64(1), cell.Land, &mt); left != nil {
		t.Errorf("an order with nothing queued previews %v, want nothing", left)
	}
}
