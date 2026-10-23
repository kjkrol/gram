package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

var (
	water = cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water}
	ice   = cell.Kind{Name: cell.Named("ice"), Cost: 1, Allows: cell.Land}
)

func TestNavigation_IceMeltingAheadRerouteWithoutSteppingIn(t *testing.T) {
	pw := newProfiledWorld(t, 5, 3, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	start, bridge, goal := pw.cellAt(0, 0), pw.cellAt(0, 1), pw.cellAt(0, 2)
	profile := steering.Steering{MaxSpeed: 64, Accel: 128, Brake: 512, V0: 16}
	pw = newProfiledWorld(t, 5, 3, start, MoveOrder{Target: goal}, profile, true)
	for x := uint32(0); x < 4; x++ {
		pw.terrain.Set(pw.cellAt(x, 1), water)
	}
	pw.terrain.Set(bridge, ice) // the straight way south, for now

	for range 3 {
		pw.tick()
	}
	if _, mt := pw.state(); mt == nil || !mt.Leg.Active || mt.Leg.To != bridge {
		t.Fatalf("after three ticks the unit is not on a leg towards the bridge: %+v", mt)
	}
	pw.terrain.Set(bridge, water)

	bridgeCentre := pw.grid.CellCenter(bridge)
	for tick := range 60 * 10 {
		_, centre, ordered := pw.tick()
		if c, _ := pw.grid.CellAt(centre); c == bridge {
			t.Fatalf("tick %d: the unit's centre is over the melted bridge at %v", tick, bridgeCentre)
		}
		if !ordered {
			if c, _ := pw.grid.CellAt(centre); c != goal {
				t.Fatalf("tick %d: order done at %v, want the goal", tick, c)
			}
			return
		}
	}
	t.Fatal("never reached the goal round by the ford")
}

func TestNavigation_GroundChangingUnderTheRouteIsNoticedBeforeTheLeg(t *testing.T) {
	pw := newProfiledWorld(t, 1, 5, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	start, far, goal := pw.cellAt(0, 0), pw.cellAt(0, 3), pw.cellAt(0, 4)
	profile := steering.Steering{MaxSpeed: 64, Accel: 128, Brake: 512, V0: 16}
	pw = newProfiledWorld(t, 1, 5, start, MoveOrder{Target: goal}, profile, true)

	pw.tick()
	if _, mt := pw.state(); mt == nil || mt.Path.Length != 4 {
		t.Fatalf("after one tick: %+v, want a four-step route", mt)
	}
	pw.terrain.Set(far, water)
	pw.tick()
	if _, mt := pw.state(); mt != nil && mt.Path.Length == 4 {
		t.Fatal("the route still runs through the water a tick after the change")
	}
	for range 60 * 3 {
		_, centre, ordered := pw.tick()
		if c, _ := pw.grid.CellAt(centre); c == far || c == goal {
			t.Fatalf("the unit got to %v across the water", c)
		}
		if !ordered {
			return
		}
	}
	t.Fatal("the unit never gave up an order it cannot carry out")
}

func TestNavigation_WeakBrakesStillComeToRestOnTheGoal(t *testing.T) {
	pw := newProfiledWorld(t, 8, 1, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	goal := pw.cellAt(7, 0)
	runs := map[string]steering.Steering{
		"firm":     {MaxSpeed: 64, Accel: 128, Brake: 256, V0: 16},
		"slipping": {MaxSpeed: 64, Accel: 128, Brake: 16, V0: 16},
	}
	brakingFrom := map[string]float64{}
	for name, profile := range runs {
		pw = newProfiledWorld(t, 8, 1, pw.cellAt(0, 0), MoveOrder{Target: goal}, profile, true)
		peak, arrived := 0.0, false
		for range 60 * 30 {
			vel, centre, ordered := pw.tick()
			peak = max(peak, vel.Value)
			if ordered && vel.Value < peak && brakingFrom[name] == 0 {
				brakingFrom[name] = pw.grid.CellCenter(goal).X - centre.X
			}
			if !ordered {
				if want := pw.grid.CellCenter(goal); centre != want {
					t.Errorf("%s: came to rest at %v, want the goal's centre %v", name, centre, want)
				}
				arrived = true
				break
			}
		}
		if !arrived {
			t.Fatalf("%s: never arrived", name)
		}
	}
	if brakingFrom["slipping"] <= brakingFrom["firm"] {
		t.Errorf("braking began %v from the goal slipping and %v firm; want the slipping unit to brake earlier", brakingFrom["slipping"], brakingFrom["firm"])
	}
}

func (pw *profiledWorld) state() (world.Base, *MoveOrder) {
	pw.q.All()
	if !pw.q.Next() {
		return world.Base{}, nil
	}
	cur := pw.q.Cursor()
	b := pw.pos.Slice(cur)[0]
	if orders := pw.orderComp.Slice(cur); orders != nil {
		o := orders[0]
		return b, &o
	}
	return b, nil
}

func TestNavigation_StuckOnForbiddenGroundKeepsTheOrder(t *testing.T) {
	pw := newProfiledWorld(t, 1, 6, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	start, goal := pw.cellAt(0, 0), pw.cellAt(0, 5)
	profile := steering.Steering{MaxSpeed: 64, Accel: 128, Brake: 512, V0: 16}
	pw = newProfiledWorld(t, 1, 6, start, MoveOrder{Target: goal}, profile, true)

	pw.terrain.Set(start, water) // the ground under it turns against it before it has set off
	for range 60 * 2 {
		vel, centre, ordered := pw.tick()
		if !ordered {
			t.Fatal("the order was given up while the unit was stuck")
		}
		if c, _ := pw.grid.CellAt(centre); c != start {
			t.Fatalf("the unit moved to %v off forbidden ground", c)
		}
		_ = vel
	}
	pw.terrain.Set(start, cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	for range 60 * 10 {
		_, centre, ordered := pw.tick()
		if !ordered {
			if c, _ := pw.grid.CellAt(centre); c != goal {
				t.Fatalf("order done at %v, want the goal", c)
			}
			return
		}
	}
	t.Fatal("never reached the goal once the ground took the unit again")
}

type slantRow struct{ At cell.ID }

func TestNavigation_CrossesABridgeRunningSlantwise(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 5, 32)
	start, goal := g.At(0, 0), g.At(4, 4)
	prepare := func(w *world.Plugin, brd *board.Plugin) {
		brd.Write().Board().SetAll(water)
		brd.Write().Board().Set(start, cell.Kind{Cost: 1, Allows: cell.Land})
		brd.Write().Board().Set(goal, cell.Kind{Cost: 1, Allows: cell.Land})
		nw, _ := grid.Direction(g, -1, -1)
		se, _ := grid.Direction(g, 1, 1)
		decks := board.NewEntities(brd, board.Shape{Size: 10},
			func(r slantRow) cell.ID { return r.At })
		decks.Define("deck",
			comp.Const(cell.Floor{Allows: cell.Land, Links: 1<<nw | 1<<se, Width: 24}),
			comp.Const(cell.Rail{Keeps: cell.Land}))
		for _, c := range []cell.ID{g.At(1, 1), g.At(2, 2), g.At(3, 3)} {
			w.Write().Seed(decks.Named("deck").Entry(slantRow{At: c}))
		}
	}
	var at goke.Comp[unit.At]
	var orderComp goke.OptComp[MoveOrder]
	var q *goke.Query
	nw := newNavWorldWith(t, 5, 5, 32, []navUnit{{
		box:     cellBox(g, start, 10),
		at:      start,
		profile: &steering.Steering{MaxSpeed: 64},
		order:   &MoveOrder{Target: goal},
	}}, prepare, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&at).Optional(&orderComp).Build()
	}})
	var here cell.ID
	for tick := range 60 * 5 {
		nw.ecs.Tick(time.Second / 60)
		for q.All(); q.Next(); {
			cur := q.Cursor()
			here = at.Slice(cur)[0].Cell
			if x, y, _ := g.Coords(here); x != y {
				t.Fatalf("tick %d: at %v, off the bridge", tick, here)
			}
		}
	}
	if here != goal {
		t.Errorf("at %v after five seconds, want across the river at %v", here, goal)
	}
}
