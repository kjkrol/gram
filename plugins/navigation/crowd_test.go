package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
)

var wall = cell.Kind{Pace: 1, Solid: true}

// xy are x, y pairs
func (rw *roadWorld) walls(xy ...uint32) {
	for i := 0; i+1 < len(xy); i += 2 {
		rw.nav.boardPlugin.Write().Board().Set(rw.at(xy[i], xy[i+1]), wall)
	}
}

func (rw *roadWorld) watch(limit time.Duration, each func(tick int) (done bool)) bool {
	for tick := 0; time.Duration(tick)*time.Second/60 < limit; tick++ {
		rw.ecs.Tick(time.Second / 60)
		if each(tick) {
			return true
		}
	}
	return false
}

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
		if stood > int(spacing.StallAfter/(time.Second/60)) {
			t.Fatalf("the traveller stood at %v for %v: it waited on the stranger, want it gone round at once", c, spacing.StallAfter)
		}
		last = c
		return o == nil && c == rw.at(9, 1)
	})
	if !arrived {
		t.Fatal("the traveller never got past the stranger")
	}
}

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

func TestCrowd_BodiesGoRoundAStrangerAndAnAllyMakesWay(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal, home := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80), geom.NewVec(5*fieldCell, 80)
	for _, c := range []struct {
		owner  command.Issuer
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
		if at, o := fw.centre(0); o != nil || f32.Hypot(at.X-goal.X, at.Y-goal.Y) > 1 {
			t.Errorf("past player %d's unit the traveller stands at %v with order %v, want at %v", c.owner, at, o != nil, goal)
		}
		at, o := fw.centre(1)
		if o != nil {
			t.Errorf("player %d's unit still has an order %+v, want it standing", c.owner, o)
		}
		// a stranger is only nudged by the collision; an ally stands aside, off the way
		if off := f32.Abs(at.Y - home.Y); c.yields != (off > 6) {
			t.Errorf("player %d's unit stands %v off the way, at %v; want aside %v", c.owner, off, at, c.yields)
		}
	}
}

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
		if at, _ := fw.centre(i); f32.Hypot(at.X-want.X, at.Y-want.Y) > 1 {
			t.Errorf("unit %d stands at %v, want %v", i, at, want)
		}
	}
}

func TestPlan_APatrolOrdersTheUnitAloneAndGoesOnOnceArrived(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	east, west := rw.at(8, 1), rw.at(1, 1)
	patrol := &writtenPlan{plan.New("navigation test patrol", plan.Loop[any]{
		Name:  "leg",
		When:  cond.Self.If(cond.Always),
		Then:  plan.Self.Do(Onward{}),
		Until: cond.Self.If(Arrived{}),
	})}
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), owner: 1, selected: true, plan: patrol, round: []cell.ID{east, west}},
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

func TestPlan_AnAlarmTakesOverThePatrolAndGivesItBack(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	east, west, gate := rw.at(8, 1), rw.at(1, 1), rw.at(4, 0)
	alarm := rw.w.Write().Effects().Named("alarm")
	guard := &writtenPlan{plan.New("navigation test guard", plan.OneOf(
		plan.Loop[any]{
			Name:  "alarm",
			When:  cond.On(rw.w).If(alarm),
			Then:  plan.Self.Do(MoveTo{Cell: gate}),
			Until: cond.Self.If(Arrived{}),
		},
		plan.Loop[any]{
			Name:  "patrol",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(Onward{}),
			Until: cond.Self.If(Arrived{}),
		},
	))}
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), owner: 1, plan: guard, round: []cell.ID{east, west}},
	})
	alarm = rw.w.Write().Effects().Named("alarm")
	walker := rw.byRow[0]
	rw.watch(10*time.Second, func(int) bool { c, _ := rw.state(walker); return c == rw.at(3, 1) })
	rw.w.Write().Carrier().Put(1, ceremony.Put(alarm).On(ceremony.World))
	if !rw.watch(10*time.Second, func(int) bool { c, _ := rw.state(walker); return c == gate }) {
		c, _ := rw.state(walker)
		t.Fatalf("under the alarm the guard stands at %v, want it at the gate %v", c, gate)
	}
	rw.w.Write().Carrier().Put(1, ceremony.Take(alarm).On(ceremony.World))
	if !rw.watch(20*time.Second, func(int) bool { c, _ := rw.state(walker); return c == east }) {
		c, _ := rw.state(walker)
		t.Errorf("the alarm lifted, the guard stands at %v, want it back on patrol at %v", c, east)
	}
}

func TestPlan_ARestOnArrivingIsOnceAndThePatrolGoesOn(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	east, west := rw.at(8, 1), rw.at(1, 1)
	guard := &writtenPlan{plan.New("navigation test resting guard", plan.OneOf(
		plan.Loop[Arrived]{
			Name: "rest",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(500 * time.Millisecond),
		},
		plan.Loop[any]{
			Name:  "patrol",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(Onward{}),
			Until: cond.Self.If(Arrived{}),
		},
	))}
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), owner: 1, plan: guard, round: []cell.ID{east, west}},
	})
	walker := rw.byRow[0]
	var reached []cell.ID
	rw.watch(30*time.Second, func(int) bool {
		c, o := rw.state(walker)
		post := c == east || c == west
		if o == nil && post && (len(reached) == 0 || reached[len(reached)-1] != c) {
			reached = append(reached, c)
		}
		return len(reached) == 3
	})
	if len(reached) != 3 {
		t.Errorf("resting on arriving, the patrol reached %v, want east, west and east again",
			reached)
	}
}

func yielderIf(role *ceremony.Part, yields bool) *ceremony.Part {
	if yields {
		return role
	}
	return nil
}

// cliffs pace a step onto their cells at half of what a step aside may leave a unit
type cliffs map[cell.ID]bool

func (c cliffs) Pace(_, to cell.ID, k cell.Kind, _ cell.Level, d cell.Domain) float64 {
	if c[to] {
		return spacing.YieldShare / 2 * k.PaceFor(d)
	}
	return k.PaceFor(d)
}

func (cliffs) Flat(_ cell.ID, k cell.Kind, _ cell.Level, d cell.Domain) float64 {
	return k.PaceFor(d)
}

func (cliffs) Sloped() bool { return true }

func TestCrowd_NeverStepsAsideIntoWaterAHoleOffACliffOrIntoAWall(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 32)
	at := func(x, y uint32) cell.ID { c := grid.At(x, y); return c }
	land := cell.Kind{Pace: 1, Allows: cell.Land}
	hazards := map[string]cell.Kind{
		"water": {Pace: 1, Allows: cell.Water},
		"hole":  {Pace: 1},
		"wall":  {Pace: 1, Solid: true},
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
			finder := spacing.NewFinder(grid, terrain, steep, spacing.OpenOccupancy{})
			coming := bodies.Body{ID: 3, At: grid.CellCenter(at(1, 2)), Half: geom.NewVec(3, 3), Cell: at(1, 2), Moving: true, Domain: cell.Land}

			cells := spacing.NewCellKeeping(finder, &cell.SingleOccupancy{})
			m := bodies.Member{ID: 7, Cell: at(2, 2), From: at(2, 2), Domain: cell.Land,
				Pos: posAt(grid, at(2, 2))}
			if o, ok := cells.StepAside(m, coming); ok != open || open && o.Target != at(2, 3) {
				t.Errorf("cells, %s on every side but open %v: stepped aside %v to %v, want only to the open (2,3)", name, open, ok, o.Target)
			}

			keep := spacing.NewBodyKeeping(finder, nil, nil)
			keep.Begin(func(dst []bodies.Body) []bodies.Body { return dst })
			c := grid.CellCenter(at(2, 2))
			m = bodies.Member{ID: 7, Cell: at(2, 2), From: at(2, 2), Domain: cell.Land,
				Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(c.X-14, c.Y-14), 28, 28)}}
			o, ok := keep.StepAside(m, coming)
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

func TestCrowd_AGameGivesItsOwnRules(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal, home := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80), geom.NewVec(5*fieldCell, 80)
	yielder := ceremony.NewPart("yielder", 0)
	forAll := act.Branch[Touch]{
		Name: "a yielder makes way for all",
		When: cond.Self.If(yielder).
			And(cond.Self.If(cond.NewWord("pushed", func(t Touch) bool { return !t.Moving && t.OtherMoving }))),
		Then: act.Self.Do(StepAside{}),
	}
	for _, c := range []struct {
		standing command.Issuer
		yields   bool
	}{{2, true}, {3, false}, {1, false}} {
		fw := newFieldWorldWith(t, 10, 5, BodySpacing, nil, []fieldUnit{
			{at: from, side: 6, order: probe.standAt(goal), owner: 1},
			{at: home, side: 6, owner: c.standing, plays: yielderIf(yielder, c.yields)},
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
		if at, o := fw.centre(0); o != nil || f32.Hypot(at.X-goal.X, at.Y-goal.Y) > 1 {
			t.Errorf("past player %d's unit the traveller stands at %v with order %v, want at %v", c.standing, at, o != nil, goal)
		}
	}
}

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

func TestCrowd_WithNoRulesTheOneInTheWayIsStruckNotPushedAlong(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80)
	home := geom.NewVec(5*fieldCell, 80)
	fw := newFieldWorldWith(t, 10, 5, BodySpacing, nil, []fieldUnit{
		{at: from, side: 6, order: probe.standAt(goal)},
		{at: home, side: 6},
	}, func(p *Plugin) { p.WithCrowd() })
	for range 30 * 60 {
		fw.ecs.Tick(time.Second / 60)
		if _, o := fw.centre(0); o == nil {
			break
		}
	}
	// half the strike's overlap and the ticks it takes to stop: 1.07
	if at, _ := fw.centre(1); f32.Hypot(at.X-home.X, at.Y-home.Y) > 1.5 {
		t.Errorf("the one in the way stands at %v, from %v: struck, not pushed along", at, home)
	}
}

func TestCrowd_NobodyIsPushedOverTheWater(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	lay := func(b testBoard, at func(x, y uint32) cell.ID) {
		for x := uint32(0); x < 10; x++ {
			b.Set(at(x, 3), cell.Kind{Pace: 1, Allows: cell.Water})
			b.Set(at(x, 4), cell.Kind{Pace: 1, Allows: cell.Water})
		}
		b.Set(at(4, 2), cell.Kind{Pace: 1, Solid: true})
		b.Set(at(6, 2), cell.Kind{Pace: 1, Solid: true})
	}
	shore := float32(3 * fieldCell)
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
