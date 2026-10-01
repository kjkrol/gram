package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Under CellSpacing a route is planned over the ground alone: it runs straight through the cell
// someone stands on.
func TestCellSpacing_RoutesAreBlindToOthers(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true},
		{start: rw.at(5, 1)},
	}
	rw = newRoadWorld(t, 10, units)
	if got := rw.nav.Spacing(); got != CellSpacing {
		t.Fatalf("units 22 a side on cells 32 keep apart by %v, want cells", got)
	}
	rw.ecs.Tick(time.Second / 60)
	_, o := rw.state(rw.byRow[0])
	if o == nil || !slices.Contains(o.Path.Steps[:o.Path.Length], rw.at(5, 1)) {
		t.Fatalf("the route %v does not run through the cell someone stands on, want it planned blind", o)
	}
}

// One standing on the road, come at by one on the move, steps off the road square to it and stays
// there; the one on the move gets through.
func TestCellSpacing_OneStandingInTheWayStepsAsideAndStays(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true},
		{start: rw.at(5, 1)},
	}
	rw = newRoadWorld(t, 10, units)
	mover, stander := rw.byRow[0], rw.byRow[1]
	gaveWay := false
	for range 60 * 8 {
		rw.ecs.Tick(time.Second / 60)
		if _, o := rw.state(stander); o != nil && o.GivingWay {
			gaveWay = true
		}
		if mc, mo := rw.state(mover); mo == nil && mc == rw.at(9, 1) {
			break
		}
	}
	for range 60 * 3 {
		rw.ecs.Tick(time.Second / 60)
	}
	if mc, mo := rw.state(mover); mo != nil || mc != rw.at(9, 1) {
		t.Fatalf("the mover stands at %v with %+v, want through at (9,1)", mc, mo)
	}
	if c, o := rw.state(stander); !gaveWay || o != nil || c != rw.at(5, 0) && c != rw.at(5, 2) {
		t.Errorf("the standing unit gave way %v and stands at %v with %+v; want it aside, square off the road, for good", gaveWay, c, o)
	}
}

// With walls beside the road nobody can give way and there is no way round: the one on the move
// waits, stalls a few times and gives its order up where it stands; the standing one never moves.
func TestCellSpacing_NoRoomToGiveWayAndNoWayRoundTheMoverGivesUp(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true},
		{start: rw.at(5, 1)},
	}
	rw = newRoadWorld(t, 10, units)
	wall := cell.Kind{Cost: 1, Solid: true}
	for _, x := range []uint32{4, 5, 6} {
		rw.nav.board.Set(rw.at(x, 0), wall)
		rw.nav.board.Set(rw.at(x, 2), wall)
	}
	mover, stander := rw.byRow[0], rw.byRow[1]
	for range 60 * 12 {
		rw.ecs.Tick(time.Second / 60)
		if c, o := rw.state(stander); c != rw.at(5, 1) || o != nil {
			t.Fatalf("the standing unit moved to %v with order %v, want it standing: nowhere to give way", c, o)
		}
		if c, o := rw.state(mover); o == nil {
			if c != rw.at(4, 1) {
				t.Errorf("gave up at %v, want (4,1), where it waited", c)
			}
			return
		}
	}
	t.Fatal("the mover never gave up within 12 s")
}

// Nobody makes way for one giving way itself; one standing steps aside square off the way of the
// one coming, or slantwise beside it with those cells held — never ahead of it — and stays there.
func TestCellSpacing_OneStandingStepsAsideOffTheWay(t *testing.T) {
	if (Touch{OtherMoving: true, OtherGivingWay: true, Ally: true}).PushedByAlly() {
		t.Error("made way for one giving way")
	}
	grid := board.DefaultGrids{}.Square(3, 3, 32)
	terrain := board.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occ := &board.SingleOccupancy{}
	k := newCellKeeping(newPathFinder(grid, terrain, nil, openOccupancy{}), occ)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	m := member{id: 7, cell: at(1, 1), from: at(1, 1), domain: cell.Land, pos: posAt(grid, at(1, 1))}
	coming := body{id: 3, at: grid.CellCenter(at(0, 1)), cell: at(0, 1), moving: true}
	o, ok := k.stepAside(m, coming)
	if !ok || !o.GivingWay || o.Target != at(1, 0) && o.Target != at(1, 2) || o.Queued != 0 {
		t.Errorf("stepped aside with %+v %v, want an order square off the way, (1,0) or (1,2), and nothing after", o, ok)
	}
	occ.Enter(at(1, 0), uid.UID64(4), cell.Land)
	occ.Enter(at(1, 2), uid.UID64(5), cell.Land)
	if o, ok := k.stepAside(m, coming); !ok || o.Target != at(0, 0) && o.Target != at(0, 2) {
		t.Errorf("with the cells across held it stepped aside to %v %v, want slantwise beside the one coming, never ahead of it", o.Target, ok)
	}
}

// posAt is a 22-unit box on c.
func posAt(grid board.Grid, c cell.ID) world.Position {
	return world.Position{AABB: board.CellAABB(grid, c, 22)}
}
