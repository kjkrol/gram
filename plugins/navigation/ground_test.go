package navigation

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

var (
	water = board.CellKind{Name: board.Named("water"), Cost: 1, Allows: board.Water}
	ice   = board.CellKind{Name: board.Named("ice"), Cost: 1, Allows: board.Land}
)

// A land unit routed over an ice bridge that melts before it gets there stops short of the water
// and goes round by the ford.
func TestNavigation_IceMeltingAheadRerouteWithoutSteppingIn(t *testing.T) {
	pw := newProfiledWorld(t, 5, 3, board.CellID(0), MoveOrder{}, steering.Steering{}, true)
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

// A step further along the route than the current leg turning to water invalidates the route
// as soon as the terrain changes; with no way round, the unit stops and gives the order up.
func TestNavigation_GroundChangingUnderTheRouteIsNoticedBeforeTheLeg(t *testing.T) {
	pw := newProfiledWorld(t, 1, 5, board.CellID(0), MoveOrder{}, steering.Steering{}, true)
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

// Weak brakes — a slipping unit — mean braking earlier, not overshooting the goal.
func TestNavigation_WeakBrakesStillComeToRestOnTheGoal(t *testing.T) {
	pw := newProfiledWorld(t, 8, 1, board.CellID(0), MoveOrder{}, steering.Steering{}, true)
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

// state reads the one entity's Base and its order, nil once the order is done.
func (pw *profiledWorld) state() (world.Base, *MoveOrder) {
	pw.q.All()
	if !pw.q.Next() {
		return world.Base{}, nil
	}
	cur := pw.q.Cursor()
	b := pw.pos.Slice(cur)[0]
	if orders := pw.order.Slice(cur); orders != nil {
		o := orders[0]
		return b, &o
	}
	return b, nil
}

// A unit standing where its domain may not — frozen in — keeps its order and moves on once the
// ground takes it again, instead of giving the order up as unreachable.
func TestNavigation_StuckOnForbiddenGroundKeepsTheOrder(t *testing.T) {
	pw := newProfiledWorld(t, 1, 6, board.CellID(0), MoveOrder{}, steering.Steering{}, true)
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
	pw.terrain.Set(start, board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
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
