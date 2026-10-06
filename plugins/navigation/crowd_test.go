package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/plan"
)

// wall is ground nobody walks.
var wall = cell.Kind{Cost: 1, Solid: true}

// walls builds walls on the cells given as x, y pairs.
func (rw *roadWorld) walls(xy ...uint32) {
	for i := 0; i+1 < len(xy); i += 2 {
		rw.nav.board.Set(rw.at(xy[i], xy[i+1]), wall)
	}
}

// watch ticks the road world for up to limit, calling each after every tick; it stops early when
// each reports it is done.
func (rw *roadWorld) watch(limit time.Duration, each func(tick int) (done bool)) bool {
	for tick := 0; time.Duration(tick)*time.Second/60 < limit; tick++ {
		rw.ecs.Tick(time.Second / 60)
		if each(tick) {
			return true
		}
	}
	return false
}

// A stranger standing on the road is not asked and never moves; the traveller goes round it at
// once, without waiting.
func TestCrowd_AStrangerStandingIsGoneRoundAtOnce(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1},
		{start: rw.at(5, 1), owner: 2},
	})
	traveller, stranger := rw.byRow[0], rw.byRow[1]
	stood, last := 0, cell.ID(0)
	arrived := rw.watch(8*time.Second, func(int) bool {
		if c, o := rw.state(stranger); c != rw.at(5, 1) || o != nil {
			t.Fatalf("the stranger moved to %v with %+v; want it standing: nobody asks a stranger", c, o)
		}
		c, o := rw.state(traveller)
		if c == last && o != nil {
			stood++
		} else {
			stood = 0
		}
		if stood > int(stallAfter/(time.Second/60)) {
			t.Fatalf("the traveller stood at %v for %v: it waited on the stranger, want it gone round at once", c, stallAfter)
		}
		last = c
		return o == nil && c == rw.at(9, 1)
	})
	if !arrived {
		t.Fatal("the traveller never got past the stranger")
	}
}

// An ally standing on the road makes way — square off the road — and stays there; the traveller
// gets through.
func TestCrowd_AnAllyStandingMakesWayAndStaysAside(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1},
		{start: rw.at(5, 1), owner: 1},
	})
	traveller, ally := rw.byRow[0], rw.byRow[1]
	aside := cell.ID(0)
	done := rw.watch(10*time.Second, func(int) bool {
		if c, _ := rw.state(ally); c != rw.at(5, 1) {
			aside = c
		}
		c, o := rw.state(traveller)
		return o == nil && c == rw.at(9, 1)
	})
	if !done {
		t.Fatalf("within 10 s the traveller did not get through; the ally stepped aside to %v", aside)
	}
	rw.watch(3*time.Second, func(int) bool { return false })
	if c, o := rw.state(ally); c != rw.at(5, 0) && c != rw.at(5, 2) || o != nil {
		t.Errorf("the ally stands at %v with %+v; want it aside, square off the road, for good", c, o)
	}
}

// An idle ally standing on the traveller's goal makes way for good; beside a stranger standing
// there the traveller stands at once.
func TestCrowd_AGoalTakenIsFreedByAnAllyAndSettledBesideAStranger(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1},
		{start: rw.at(9, 1), owner: 1},
	})
	traveller, ally := rw.byRow[0], rw.byRow[1]
	if !rw.watch(10*time.Second, func(int) bool { c, o := rw.state(traveller); return o == nil && c == rw.at(9, 1) }) {
		t.Fatal("the ally never freed the goal")
	}
	rw.watch(3*time.Second, func(int) bool { return false })
	if c, o := rw.state(ally); c == rw.at(9, 1) || o != nil {
		t.Errorf("the ally is at %v with %+v; want it off the goal for good", c, o)
	}

	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1},
		{start: rw.at(9, 1), owner: 2},
	})
	traveller, stranger := rw.byRow[0], rw.byRow[1]
	if !rw.watch(10*time.Second, func(int) bool { _, o := rw.state(traveller); return o == nil }) {
		t.Fatal("the traveller never settled beside the stranger on its goal")
	}
	if c, _ := rw.state(traveller); rw.grid.Distance(c, rw.at(9, 1)) > 1.5 {
		t.Errorf("the traveller settled at %v, want beside its goal", c)
	}
	if c, o := rw.state(stranger); c != rw.at(9, 1) || o != nil {
		t.Errorf("the stranger moved to %v with %+v; want it standing", c, o)
	}
}

// Two strangers head on in a corridor with one passing place: the first waits, the other goes
// round by the passing place, and both get through — no endless rerouting into each other.
func TestCrowd_TwoStrangersHeadOnInACorridorBothGetThrough(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1},
		{start: rw.at(9, 1), target: rw.at(0, 1), ordered: true, owner: 2},
	})
	for x := uint32(0); x < 10; x++ {
		rw.walls(x, 2)
		if x < 4 || x > 5 {
			rw.walls(x, 0)
		}
	}
	a, b := rw.byRow[0], rw.byRow[1]
	if !rw.watch(30*time.Second, func(int) bool {
		ca, oa := rw.state(a)
		cb, ob := rw.state(b)
		return oa == nil && ob == nil && ca == rw.at(9, 1) && cb == rw.at(0, 1)
	}) {
		ca, oa := rw.state(a)
		cb, ob := rw.state(b)
		t.Fatalf("within 30 s: one at %v (%v), the other at %v (%v); want both through", ca, oa != nil, cb, ob != nil)
	}
}

// Under BodySpacing a stranger standing in the way never makes way: the traveller goes round it;
// an ally makes way and stays aside.
func TestCrowd_BodiesGoRoundAStrangerAndAnAllyMakesWay(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal, home := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80), geom.NewVec(5*fieldCell, 80)
	for _, c := range []struct {
		owner  control.PlayerID
		yields bool
	}{{2, false}, {1, true}} {
		fw := newFieldWorld(t, 10, 5, BodySpacing, nil, []fieldUnit{
			{at: from, side: 6, order: probe.standAt(goal), owner: 1},
			{at: home, side: 6, owner: c.owner},
		})
		gaveWay := false
		for tick := 0; tick < 20*60; tick++ {
			fw.ecs.Tick(time.Second / 60)
			if _, o := fw.centre(1); o != nil && o.GivingWay {
				gaveWay = true
			}
		}
		if gaveWay != c.yields {
			t.Errorf("owned by player %d the one standing gave way %v, want %v", c.owner, gaveWay, c.yields)
		}
		if at, o := fw.centre(0); o != nil || math.Hypot(at.X-goal.X, at.Y-goal.Y) > 1 {
			t.Errorf("past player %d's unit the traveller stands at %v with order %v, want at %v", c.owner, at, o != nil, goal)
		}
		at, o := fw.centre(1)
		if o != nil {
			t.Errorf("player %d's unit still has an order %+v, want it standing", c.owner, o)
		}
		// struck, a stranger is only nudged by the collision; an ally stands aside, off the way
		if off := math.Abs(at.Y - home.Y); c.yields != (off > 6) {
			t.Errorf("player %d's unit stands %v off the way, at %v; want aside %v", c.owner, off, at, c.yields)
		}
	}
}

// Under BodySpacing two strangers head on pass each other and both arrive.
func TestCrowd_BodiesOfTwoStrangersHeadOnPass(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	goal0, goal1 := geom.NewVec(9*fieldCell+16, 80), geom.NewVec(16, 80)
	fw := newFieldWorld(t, 10, 5, BodySpacing, nil, []fieldUnit{
		{at: goal1, side: 6, order: probe.standAt(goal0), owner: 1},
		{at: goal0, side: 6, order: probe.standAt(goal1), owner: 2},
	})
	if settled, _ := fw.run(15 * time.Second); !settled {
		t.Fatal("the two strangers have not arrived")
	}
	for i, want := range []geom.Vec{goal0, goal1} {
		if at, _ := fw.centre(i); math.Hypot(at.X-want.X, at.Y-want.Y) > 1 {
			t.Errorf("unit %d stands at %v, want %v", i, at, want)
		}
	}
}

// A unit's plan orders it as a player would, for itself alone: a patrol along the road, each way
// once it is told Arrived. The player's other unit, selected, is never sent.
func TestPlan_APatrolOrdersTheUnitAloneAndGoesOnOnceArrived(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	east, west := rw.at(8, 1), rw.at(1, 1)
	patrol := &writtenPlan{"navigation test patrol", func(a *plan.Actor) rule.Step {
		return a.Steps(
			a.Order(MoveTo{Cell: east}).Until[Arrived](),
			a.Order(MoveTo{Cell: west}).Until[Arrived](),
		)
	}}
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), owner: 1, selected: true, plan: patrol},
		{start: rw.at(0, 0), owner: 1, selected: true},
	})
	walker, bystander := rw.byRow[0], rw.byRow[1]
	var reached []cell.ID
	rw.watch(20*time.Second, func(int) bool {
		if c, o := rw.state(bystander); c != rw.at(0, 0) || o != nil {
			t.Fatalf("the bystander stands at %v with %+v; want it never sent: the patrol orders itself alone", c, o)
		}
		c, o := rw.state(walker)
		if o == nil && (c == east || c == west) && (len(reached) == 0 || reached[len(reached)-1] != c) {
			reached = append(reached, c)
		}
		return len(reached) == 3
	})
	if len(reached) != 3 || reached[0] != east || reached[1] != west || reached[2] != east {
		t.Errorf("the patrol reached %v, want east %v, west %v, east again", reached, east, west)
	}
}

// cliffs are the cells a step into is a cliff, steeper than yieldClimb.
type cliffs map[cell.ID]bool

func (c cliffs) Climb(_, to cell.ID, _ cell.Domain) float64 {
	if c[to] {
		return 2 * yieldClimb
	}
	return 1
}
func (cliffs) Least(cell.Domain) float64 { return 1 }

// One standing is never stepped aside into water or a hole, down a cliff or into a wall, under
// either spacing: with such ground on every side off the way of the one coming, it has no order
// and stays; with one side open, it steps there.
func TestCrowd_NeverStepsAsideIntoWaterAHoleOffACliffOrIntoAWall(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 32)
	at := func(x, y uint32) cell.ID { c := grid.CellIndex(x, y); return c }
	land := cell.Kind{Cost: 1, Allows: cell.Land}
	hazards := map[string]cell.Kind{
		"water": {Cost: 1, Allows: cell.Water},
		"hole":  {Cost: 1},
		"wall":  {Cost: 1, Solid: true},
		"cliff": land,
	}
	aside := []cell.ID{at(2, 1), at(2, 3), at(1, 1), at(1, 3)} // off the way of one coming from (1,2)
	for name, hazard := range hazards {
		for _, open := range []bool{false, true} {
			terrain := cell.NewTerrainMap()
			terrain.SetAll(land)
			steep := cliffs{}
			for _, c := range aside {
				if open && c == at(2, 3) {
					continue
				}
				terrain.Set(c, hazard)
				if name == "cliff" {
					steep[c] = true
				}
			}
			finder := newPathFinder(grid, terrain, steep, openOccupancy{})
			coming := body{id: 3, at: grid.CellCenter(at(1, 2)), half: geom.NewVec(3, 3), cell: at(1, 2), moving: true, domain: cell.Land}

			cells := newCellKeeping(finder, &cell.SingleOccupancy{})
			m := member{id: 7, cell: at(2, 2), from: at(2, 2), domain: cell.Land, pos: posAt(grid, at(2, 2))}
			if o, ok := cells.stepAside(m, coming); ok != open || open && o.Target != at(2, 3) {
				t.Errorf("cells, %s on every side but open %v: stepped aside %v to %v, want only to the open (2,3)", name, open, ok, o.Target)
			}

			bodies := newBodyKeeping(finder, nil, nil)
			bodies.begin(func(dst []body) []body { return dst })
			c := grid.CellCenter(at(2, 2))
			m = member{id: 7, cell: at(2, 2), from: at(2, 2), domain: cell.Land,
				pos: world.Position{AABB: plane.NewAABB(geom.NewVec(c.X-14, c.Y-14), 28, 28)}}
			o, ok := bodies.stepAside(m, coming)
			if ok != open {
				t.Errorf("bodies, %s on every side but open %v: stepped aside %v, to %v", name, open, ok, o.Spot)
				continue
			}
			if cell, _ := grid.CellAt(o.Spot); open && cell != at(2, 3) {
				t.Errorf("bodies, %s but (2,3): stepped aside to %v in %v, want into the open (2,3)", name, o.Spot, cell)
			}
		}
	}
}

// The rules of the crowd read a Touch: who makes way, who stops among its group, who goes past,
// who goes round.
func TestTouch_TheRulesOfTheCrowd(t *testing.T) {
	moving := Touch{Moving: true, LastGoal: true}
	for _, c := range []struct {
		name                                       string
		touch                                      Touch
		pushed, reached, clears, blocks, goalTaken bool
	}{
		{"an idle ally pushed", Touch{OtherMoving: true, Ally: true}, true, false, false, false, false},
		{"an idle stranger pushed", Touch{OtherMoving: true}, false, false, false, false, false},
		{"one of the group arrived, pushed by its own", Touch{OtherMoving: true, Ally: true, Groupmate: true}, false, false, false, false, false},
		{"pushed by one giving way", Touch{OtherMoving: true, OtherGivingWay: true, Ally: true}, false, false, false, false, false},
		{"on the move at one of its group arrived", with(moving, func(t *Touch) { t.Ally, t.Groupmate = true, true }), false, true, false, false, false},
		{"on the move, not at its last goal, at one of its group", with(moving, func(t *Touch) { t.Ally, t.Groupmate, t.LastGoal = true, true, false }), false, false, false, true, false},
		{"on the move at an idle ally with room", with(moving, func(t *Touch) { t.Ally, t.Room = true, true }), false, false, true, false, false},
		{"on the move at an idle ally with no room", with(moving, func(t *Touch) { t.Ally = true }), false, false, false, true, false},
		{"on the move at an idle ally with no room on its goal", with(moving, func(t *Touch) { t.Ally, t.OnMyGoal = true, true }), false, false, false, true, true},
		{"on the move at an idle ally with room on its goal", with(moving, func(t *Touch) { t.Ally, t.OnMyGoal, t.Room = true, true, true }), false, false, true, false, false},
		{"on the move at a stranger on its goal", with(moving, func(t *Touch) { t.OnMyGoal = true }), false, false, false, true, true},
		{"on the move at one on the move", with(moving, func(t *Touch) { t.OtherMoving, t.Ally = true, true }), false, false, false, true, false},
	} {
		got := [5]bool{c.touch.PushedByAlly(), c.touch.ReachedTheGroup(), c.touch.ClearsTheWay(), c.touch.Blocks(), c.touch.GoalTaken()}
		if want := [5]bool{c.pushed, c.reached, c.clears, c.blocks, c.goalTaken}; got != want {
			t.Errorf("%s: pushed, reached, clears, blocks, goal taken %v, want %v", c.name, got, want)
		}
	}
	head := Touch{Self: 1, Other: 2, Moving: true, OtherMoving: true, HeadOn: true}
	if !head.WaitsFirst() || (Touch{Self: 2, Other: 1, Moving: true, OtherMoving: true, HeadOn: true}).WaitsFirst() {
		t.Error("of two head on the lower id does not wait first, or the other does too")
	}
	if head.WaitedOut = true; head.WaitsFirst() {
		t.Error("the first waits on after its Hold ran out")
	}
}

// with is t changed by fn.
func with(t Touch, fn func(*Touch)) Touch {
	fn(&t)
	return t
}

// A game gives its own rules in place of the crowd's, narrowed by tags as any rule: here player 2's
// units make way for anyone on the move, strangers too, and nobody else makes way at all.
func TestCrowd_AGameGivesItsOwnRules(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal, home := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80), geom.NewVec(5*fieldCell, 80)
	forAll := rule.Then[Touch]("player 2 makes way for all", rule.Self(owner.Of(2)), rule.If(func(t Touch) bool { return !t.Moving && t.OtherMoving }, rule.Order(StepAside{})))
	for _, c := range []struct {
		standing control.PlayerID
		yields   bool
	}{{2, true}, {3, false}, {1, false}} {
		fw := newFieldWorldWith(t, 10, 5, BodySpacing, nil, []fieldUnit{
			{at: from, side: 6, order: probe.standAt(goal), owner: 1},
			{at: home, side: 6, owner: c.standing},
		}, func(p *Plugin) { p.WithCrowd(forAll, goRound()) })
		gaveWay := false
		for tick := 0; tick < 20*60; tick++ {
			fw.ecs.Tick(time.Second / 60)
			if _, o := fw.centre(1); o != nil && o.GivingWay {
				gaveWay = true
			}
		}
		if gaveWay != c.yields {
			t.Errorf("player %d's unit standing gave way %v, want %v", c.standing, gaveWay, c.yields)
		}
		if at, o := fw.centre(0); o != nil || math.Hypot(at.X-goal.X, at.Y-goal.Y) > 1 {
			t.Errorf("past player %d's unit the traveller stands at %v with order %v, want at %v", c.standing, at, o != nil, goal)
		}
	}
}

// With no rules at all nobody makes way and nobody is gone round on purpose: the one on the move
// is left to navigation's last word — stalled, it plans afresh, and it ends its order.
func TestCrowd_WithNoRulesTheOrderStillEnds(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal, home := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80), geom.NewVec(5*fieldCell, 80)
	fw := newFieldWorldWith(t, 10, 5, BodySpacing, nil, []fieldUnit{
		{at: from, side: 6, order: probe.standAt(goal)},
		{at: home, side: 6},
	}, func(p *Plugin) { p.WithCrowd() })
	for tick := 0; tick < 30*60; tick++ {
		fw.ecs.Tick(time.Second / 60)
		if _, o := fw.centre(1); o != nil {
			t.Fatalf("tick %d: the one standing got the order %+v, want none: no rule makes way", tick, o)
		}
		if _, o := fw.centre(0); o == nil {
			return
		}
	}
	t.Fatal("the one on the move still has its order after 30 s")
}

// One standing at the water's edge with no room to step aside — walls either side — is never
// pushed over the water, with the rules of the crowd or with none: the collision holds it on its
// ground as at a wall. With the rules the one coming for its place stands beside it.
func TestCrowd_NobodyIsPushedOverTheWater(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	lay := func(b *board.Board, at func(x, y uint32) cell.ID) {
		for x := uint32(0); x < 10; x++ {
			b.Set(at(x, 3), cell.Kind{Cost: 1, Allows: cell.Water})
			b.Set(at(x, 4), cell.Kind{Cost: 1, Allows: cell.Water})
		}
		b.Set(at(4, 2), cell.Kind{Cost: 1, Solid: true})
		b.Set(at(6, 2), cell.Kind{Cost: 1, Solid: true})
	}
	shore := float64(3 * fieldCell)
	stander := geom.NewVec(5*fieldCell+16, shore-15) // a box 28 a side, a pixel off the water
	for _, rules := range []bool{true, false} {
		fw := newFieldWorldWith(t, 10, 5, BodySpacing, lay, []fieldUnit{
			{at: geom.NewVec(stander.X, 16), side: 6, order: probe.standAt(stander)},
			{at: stander, side: 28},
		}, func(p *Plugin) {
			if !rules {
				p.WithCrowd()
			}
		})
		for tick := 0; tick < 20*60; tick++ {
			fw.ecs.Tick(time.Second / 60)
			if at, _ := fw.centre(1); at.Y+14 > shore+1e-6 {
				t.Fatalf("rules %v, tick %d: the one standing reaches %v, over the water from %v", rules, tick, at.Y+14, shore)
			}
		}
		if _, o := fw.centre(0); rules && o != nil {
			t.Errorf("the one coming still has its order %+v, want it standing beside its place taken", o)
		}
	}
}
