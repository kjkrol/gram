package navigation

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/uid"
)

// hill is level ground with a few cells raised, priced by the topography's DefaultClimbing over
// steps of 10: what a Map's Climb and Least say.
type hill map[cell.ID]float64

func (h hill) Least(d cell.Domain) float64 {
	if !relief.DefaultClimbing.Feels(d) {
		return 1
	}
	return relief.DefaultClimbing.Least()
}

func (h hill) Climb(from, to cell.ID, d cell.Domain) float64 {
	c := relief.DefaultClimbing
	if !c.Feels(d) {
		return 1
	}
	return c.Factor((h[to] - h[from]) / 10)
}

// A walker goes round a hill across its way, a flyer straight over it.
func TestFindPath_GoesRoundAHillUnlessItFlies(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land | cell.Air})
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	h := hill{}
	for y := uint32(0); y < 4; y++ {
		h[at(3, y)] = 20 // a ridge across the middle, open at the bottom row
	}
	pf := newPathFinder(grid, terrain, h, &cell.MultipleOccupancy{})

	walk, ok := pf.findPath(uid.UID64(1), cell.Land, at(0, 1), at(6, 1))
	if !ok {
		t.Fatal("no way for the walker")
	}
	for _, s := range walk.Steps[:walk.Length] {
		if h[s] > 0 {
			t.Fatalf("the walker's route climbs the ridge at %v", s)
		}
	}
	fly, ok := pf.findPath(uid.UID64(2), cell.Air, at(0, 1), at(6, 1))
	if !ok || fly.Length != 6 {
		t.Errorf("the flyer: ok=%v, %d steps, want 6 straight over", ok, fly.Length)
	}
}

// Where the ground off a road costs 2.5 times the road, a walker goes round by the road rather
// than straight across; where it costs as much, straight across.
func TestFindPath_TakesTheRoadRoundWhereTheGroundCostsMore(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 5, 10)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	road := cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land}
	for _, c := range []struct {
		ground   float64
		roadOnly bool
	}{{2.5, true}, {1, false}} {
		terrain := cell.NewTerrainMap()
		terrain.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: c.ground, Allows: cell.Land})
		roadCells := map[cell.ID]bool{}
		for x := range uint32(7) {
			roadCells[at(x, 0)] = true
		}
		for y := range uint32(3) {
			roadCells[at(0, y)], roadCells[at(6, y)] = true, true
		}
		for rc := range roadCells {
			terrain.Set(rc, road)
		}
		pf := newPathFinder(grid, terrain, nil, &cell.MultipleOccupancy{})
		path, ok := pf.findPath(uid.UID64(1), cell.Land, at(0, 2), at(6, 2))
		if !ok {
			t.Fatalf("grass at %v: no way", c.ground)
		}
		off := 0
		for _, s := range path.Steps[:path.Length] {
			if !roadCells[s] {
				off++
			}
		}
		switch {
		case c.roadOnly && off > 0:
			t.Errorf("grass at %v: %d of %d steps off the road, want the road round", c.ground, off, path.Length)
		case !c.roadOnly && path.Length != 6:
			t.Errorf("grass at %v: %d steps, want 6 straight across", c.ground, path.Length)
		}
	}
}

// roadOver lays a road as a way over the grass from cell to cell, linked both ways like a game's
// layout.
func roadOver(brd *board.Board, road cell.Kind, cells ...cell.ID) {
	for i := range cells {
		if w := brd.Way(cells[i]); !w.Runs() {
			brd.SetWay(cells[i], cell.Way{Kind: road, Width: 4})
		}
		if i == 0 {
			continue
		}
		a, b := cells[i-1], cells[i]
		for _, l := range [2][2]cell.ID{{a, b}, {b, a}} {
			if bit, ok := grid.Link(brd.Grid, l[0], l[1]); ok {
				w := brd.Way(l[0])
				w.Links |= bit
				brd.SetWay(l[0], w)
			}
		}
	}
}

// A road round a bend is followed round it: a diagonal step between two of its cells across the
// bend cuts the corner over the grass beside the road and costs the grass, not the road.
func TestFindPath_FollowsARoadRoundItsBendRatherThanCuttingTheCorner(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 5, 10)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land})
	road := cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land}
	var cells []cell.ID
	for x := range uint32(5) {
		cells = append(cells, at(x, 0))
	}
	for y := uint32(1); y < 5; y++ {
		cells = append(cells, at(4, y))
	}
	roadOver(brd, road, cells...)
	pf := newPathFinder(grid, brd, nil, &cell.MultipleOccupancy{})
	if got, _ := pf.price(at(3, 0), at(4, 1), brd.Kind(at(4, 1)), cell.Land); math.Abs(got-2*math.Sqrt2) > 1e-9 {
		t.Errorf("the diagonal across the bend costs %v, want the grass's 2√2", got)
	}
	path, ok := pf.findPath(uid.UID64(1), cell.Land, at(0, 0), at(4, 4))
	if !ok || path.Length != 8 {
		t.Fatalf("ok=%v, %d steps, want the 8 of the road round its bend", ok, path.Length)
	}
	prev := at(0, 0)
	for _, s := range path.Steps[:path.Length] {
		if !brd.Way(s).Runs() || grid.NeighborCost(prev, s) != 1 {
			t.Errorf("the route steps to %v off the road or across a corner", s)
		}
		prev = s
	}
}

// A road laid slantwise is taken along its links at its own cost; the same cells unlinked are cut
// across at the grass's.
func TestFindPath_TakesADiagonalRoadAlongItsLinks(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	road := cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land}
	for _, linked := range []bool{true, false} {
		brd := board.NewBoard(grid)
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land})
		var cells []cell.ID
		for i := range uint32(5) {
			cells = append(cells, at(i, i))
		}
		if linked {
			roadOver(brd, road, cells...)
		} else {
			for _, c := range cells {
				brd.SetWay(c, cell.Way{Kind: road, Width: 4})
			}
		}
		pf := newPathFinder(grid, brd, nil, &cell.MultipleOccupancy{})
		want := math.Sqrt2
		if !linked {
			want = 2 * math.Sqrt2
		}
		if got, _ := pf.price(at(0, 0), at(1, 1), brd.Kind(at(1, 1)), cell.Land); math.Abs(got-want) > 1e-9 {
			t.Errorf("linked %v: the slantwise step costs %v, want %v", linked, got, want)
		}
		if path, ok := pf.findPath(uid.UID64(1), cell.Land, at(0, 0), at(4, 4)); linked && (!ok || path.Length != 4) {
			t.Errorf("along the links: ok=%v, %d steps, want 4 slantwise", ok, path.Length)
		}
	}
}

// A slantwise step beside a bridge crosses the water under it: a walker may not take it, a boat
// may.
func TestFindPath_CutsNoCornerOverTheWaterBesideABridge(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 3, 10)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	brd.Set(at(1, 1), cell.Kind{Name: cell.Named("river"), Cost: 1, Allows: cell.Water})
	bridge := cell.Kind{Name: cell.Named("bridge"), Cost: 1, Allows: cell.Land}
	brd.SetCrossing(at(1, 1), cell.Crossing{Way: cell.Way{Kind: bridge, Width: 4}})
	pf := newPathFinder(grid, brd, nil, &cell.MultipleOccupancy{})
	if _, ok := pf.price(at(0, 0), at(1, 1), brd.Kind(at(1, 1)), cell.Land); ok {
		t.Error("a walker may cut the corner onto the bridge over the river")
	}
	if _, ok := pf.price(at(0, 1), at(1, 1), brd.Kind(at(1, 1)), cell.Land); !ok {
		t.Error("a walker may not step straight onto the bridge")
	}
	if cost, ok := pf.price(at(0, 0), at(1, 1), brd.Kind(at(1, 1)), cell.Water); !ok || math.Abs(cost-math.Sqrt2) > 1e-9 {
		t.Errorf("a boat's slantwise step onto the river under the bridge costs %v, ok %v; want √2", cost, ok)
	}
}

// A Graded road over the ridge costs its own price alone, so the walker takes it straight over;
// the same road ungraded is priced by the slope and the walker goes round.
func TestFindPath_AGradedRoadIsNotPricedByTheSlope(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 5, 10)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	h := hill{}
	for y := uint32(0); y < 4; y++ {
		h[at(3, y)] = 20
	}
	for _, graded := range []bool{true, false} {
		terrain := cell.NewTerrainMap()
		terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
		road := cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land, Graded: graded}
		for x := range uint32(7) {
			terrain.Set(at(x, 1), road)
		}
		pf := newPathFinder(grid, terrain, h, &cell.MultipleOccupancy{})
		want := 1.0
		if !graded {
			want = relief.DefaultClimbing.Factor(2)
		}
		if got, _ := pf.price(at(2, 1), at(3, 1), road, cell.Land); math.Abs(got-want) > 1e-9 {
			t.Errorf("graded %v: the step up the ridge costs %v, want %v", graded, got, want)
		}
		path, ok := pf.findPath(uid.UID64(1), cell.Land, at(0, 1), at(6, 1))
		if !ok {
			t.Fatalf("graded %v: no way", graded)
		}
		over := 0
		for _, s := range path.Steps[:path.Length] {
			if h[s] > 0 {
				over++
			}
		}
		if graded && (path.Length != 6 || over != 1) {
			t.Errorf("graded: %d steps, %d over the ridge, want 6 straight over it by the road", path.Length, over)
		}
		if !graded && over > 0 {
			t.Error("ungraded: the walker went over the ridge, want round it")
		}
	}
}
