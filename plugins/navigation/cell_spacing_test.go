package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
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

// One standing on the road, come at by one on the move, steps off the road square to it, stands
// aside a second and comes back; the one on the move keeps its route and goes through.
func TestCellSpacing_OneStandingInTheWayGivesWayAndComesBack(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true},
		{start: rw.at(5, 1)},
	}
	rw = newRoadWorld(t, 10, units)
	mover, stander := rw.byRow[0], rw.byRow[1]
	var first []board.CellID
	gaveWay, aside, home := false, board.CellID(0), false
	for tick := range 60 * 8 {
		rw.ecs.Tick(time.Second / 60)
		cell, o := rw.state(stander)
		if o != nil && o.GivingWay {
			gaveWay = true
		}
		if gaveWay && cell != rw.at(5, 1) {
			aside = cell
		}
		if aside != 0 && cell == rw.at(5, 1) && o == nil {
			home = true
		}
		mc, mo := rw.state(mover)
		if mo != nil {
			steps := append([]board.CellID(nil), mo.Path.Steps[:mo.Path.Length]...)
			if first == nil {
				first = steps
			} else if mo.Path.Index == 0 && !equalSteps(steps, first) {
				t.Fatalf("tick %d: the mover planned again: %v, want its first route %v kept", tick, steps, first)
			}
		}
		if mo == nil && mc == rw.at(9, 1) && home {
			if aside != rw.at(5, 0) && aside != rw.at(5, 2) {
				t.Errorf("the standing unit stepped aside to %v, want square off the road, (5,0) or (5,2)", aside)
			}
			return
		}
	}
	t.Fatalf("within 8 s: gave way %v, aside %v, home %v, mover at %v", gaveWay, aside, home, func() board.CellID { c, _ := rw.state(mover); return c }())
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
	wall := board.CellKind{Cost: 1, Solid: true}
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

// Nobody gives way to one giving way.
func TestCellSpacing_NobodyGivesWayToOneGivingWay(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, 32)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occ := &board.SingleOccupancy{}
	k := newCellKeeping(newPathFinder(grid, terrain, nil, openOccupancy{}), occ)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	m := member{id: 7, cell: at(1, 1), from: at(1, 1), domain: board.Land, pos: posAt(grid, at(1, 1))}
	if _, ok := k.yield(m, []press{{other: 3, cell: at(0, 1), way: geom.NewVec(1, 0), givingWay: true}}); ok {
		t.Error("gave way to one giving way")
	}
	o, ok := k.yield(m, []press{{other: 3, cell: at(0, 1), way: geom.NewVec(1, 0)}})
	if !ok || !o.GivingWay || o.Target != at(1, 0) && o.Target != at(1, 2) || o.Queued != 1 || o.Waypoints[0].Cell != at(1, 1) {
		t.Errorf("gave way with %+v %v, want an order square off the way, (1,0) or (1,2), and home queued", o, ok)
	}
	occ.Enter(at(1, 0), uid.UID64(4), board.Land)
	occ.Enter(at(1, 2), uid.UID64(5), board.Land)
	if o, ok := k.yield(m, []press{{other: 3, cell: at(0, 1), way: geom.NewVec(1, 0)}}); !ok || o.Target != at(0, 0) && o.Target != at(0, 2) {
		t.Errorf("with the cells across held it gave way to %v %v, want slantwise beside the one coming, never ahead of it", o.Target, ok)
	}
}

// posAt is a 22-unit box on c.
func posAt(grid board.Grid, c board.CellID) world.Position {
	return world.Position{AABB: board.CellAABB(grid, c, 22)}
}
