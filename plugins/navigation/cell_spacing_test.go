package navigation

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestCellSpacing_RoutesAreBlindToOthers(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true},
		{start: rw.at(5, 1)},
	}
	rw = newRoadWorld(t, 10, units)
	if got := rw.nav.read.Spacing(); got != CellSpacing {
		t.Fatalf("units 22 a side on cells 32 keep apart by %v, want cells", got)
	}
	rw.ecs.Tick(time.Second / 60)
	_, o := rw.state(rw.byRow[0])
	if o == nil || !slices.Contains(o.Path.Steps[:o.Path.Length], rw.at(5, 1)) {
		t.Fatalf("the route %v does not run through the cell someone stands on, want it planned blind", o)
	}
}

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

func TestCellSpacing_NoRoomToGiveWayAndNoWayRoundTheMoverGivesUp(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true},
		{start: rw.at(5, 1)},
	}
	rw = newRoadWorld(t, 10, units)
	wall := cell.Kind{Cost: 1, Solid: true}
	for _, x := range []uint32{4, 5, 6} {
		rw.nav.boardPlugin.Write().Board().Set(rw.at(x, 0), wall)
		rw.nav.boardPlugin.Write().Board().Set(rw.at(x, 2), wall)
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

func TestCellSpacing_OneStandingStepsAsideOffTheWay(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 3, 32)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occ := &cell.SingleOccupancy{}
	k := spacing.NewCellKeeping(spacing.NewFinder(grid, terrain, nil, spacing.OpenOccupancy{}), occ)
	at := func(x, y uint32) cell.ID { c := grid.At(x, y); return c }
	m := bodies.Member{ID: 7, Cell: at(1, 1), From: at(1, 1), Domain: cell.Land, Pos: posAt(grid, at(1, 1))}
	coming := bodies.Body{ID: 3, At: grid.CellCenter(at(0, 1)), Cell: at(0, 1), Moving: true}
	o, ok := k.StepAside(m, coming)
	if !ok || !o.GivingWay || o.Target != at(1, 0) && o.Target != at(1, 2) || o.Queued != 0 {
		t.Errorf("stepped aside with %+v %v, want an order square off the way, (1,0) or (1,2), and nothing after", o, ok)
	}
	occ.Enter(at(1, 0), false, uid.UID64(4), cell.Land)
	occ.Enter(at(1, 2), false, uid.UID64(5), cell.Land)
	if o, ok := k.StepAside(m, coming); !ok || o.Target != at(0, 0) && o.Target != at(0, 2) {
		t.Errorf("with the cells across held it stepped aside to %v %v, want slantwise beside the one coming, never ahead of it", o.Target, ok)
	}
}

func posAt(grid grid.Grid, c cell.ID) world.Position {
	return world.Position{AABB: cellBox(grid, c, 22)}
}
