package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world/act"
)

// wall is ground nobody walks.
var wall = board.CellKind{Cost: 1, Solid: true}

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
func TestCourtesy_AStrangerStandingIsGoneRoundAtOnce(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1, courteous: true},
		{start: rw.at(5, 1), owner: 2, courteous: true},
	})
	traveller, stranger := rw.byRow[0], rw.byRow[1]
	stood, last := 0, board.CellID(0)
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

// An ally standing on the road is asked and makes way — square off the road — then comes back;
// the traveller keeps to the road.
func TestCourtesy_AnAllyStandingMakesWayAndComesBack(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1, courteous: true},
		{start: rw.at(5, 1), owner: 1, courteous: true},
	})
	traveller, ally := rw.byRow[0], rw.byRow[1]
	aside, offRoad := board.CellID(0), false
	done := rw.watch(10*time.Second, func(int) bool {
		if c, _ := rw.state(ally); c != rw.at(5, 1) {
			aside = c
		}
		c, o := rw.state(traveller)
		if _, y, _ := rw.grid.Coords(c); y != 1 {
			offRoad = true
		}
		ac, ao := rw.state(ally)
		return o == nil && c == rw.at(9, 1) && ac == rw.at(5, 1) && ao == nil && aside != 0
	})
	if !done {
		t.Fatalf("within 10 s: the ally stepped aside to %v; want it aside and home again, the traveller through", aside)
	}
	if aside != rw.at(5, 0) && aside != rw.at(5, 2) {
		t.Errorf("the ally stepped aside to %v, want square off the road", aside)
	}
	if offRoad {
		t.Error("the traveller left the road: it went round an ally that made way")
	}
}

// An idle ally standing on the traveller's goal is asked to free it and steps off for good; a
// stranger standing there is not asked, and the traveller stands beside it at once.
func TestCourtesy_AGoalTakenIsFreedByAnAllyAndSettledBesideAStranger(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1, courteous: true},
		{start: rw.at(9, 1), owner: 1, courteous: true},
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
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1, courteous: true},
		{start: rw.at(9, 1), owner: 2, courteous: true},
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

// Two strangers head on in a corridor with one passing place: one waits, the other goes round
// by the passing place, and both get through — no endless rerouting into each other.
func TestCourtesy_TwoStrangersHeadOnInACorridorBothGetThrough(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1, courteous: true},
		{start: rw.at(9, 1), target: rw.at(0, 1), ordered: true, owner: 2, courteous: true},
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

// Two of one group blocking each other in a corridor, each nearer the other's goal, swap goals:
// both stand on a goal of the group, neither squeezing past.
func TestCourtesy_AGroupSwapsGoalsWhenThatShortensBoth(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(3, 1), target: rw.at(8, 1), ordered: true, owner: 1, courteous: true, group: 7},
		{start: rw.at(4, 1), target: rw.at(1, 1), ordered: true, owner: 1, courteous: true, group: 7},
	})
	for x := uint32(0); x < 10; x++ {
		rw.walls(x, 0, x, 2)
	}
	a, b := rw.byRow[0], rw.byRow[1]
	if !rw.watch(15*time.Second, func(int) bool {
		ca, oa := rw.state(a)
		cb, ob := rw.state(b)
		return oa == nil && ob == nil && ca == rw.at(1, 1) && cb == rw.at(8, 1)
	}) {
		ca, _ := rw.state(a)
		cb, _ := rw.state(b)
		t.Fatalf("within 15 s they stand at %v and %v; want the goals swapped: (1,1) and (8,1)", ca, cb)
	}
}

// An ally with no room beside asks the ally where it could step on: that one makes way, then it
// does, and the traveller gets through; both come back.
func TestCourtesy_InACrowdTheAskIsPassedOn(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, owner: 1, courteous: true},
		{start: rw.at(5, 1), owner: 1, courteous: true},
		{start: rw.at(5, 0), owner: 1, courteous: true},
		{start: rw.at(5, 2), owner: 1, courteous: true},
	})
	rw.walls(4, 0, 4, 2)
	traveller := rw.byRow[0]
	if !rw.watch(15*time.Second, func(int) bool { c, o := rw.state(traveller); return o == nil && c == rw.at(9, 1) }) {
		t.Fatal("within 15 s the traveller did not get through the crowd")
	}
	homes := map[int]board.CellID{1: rw.at(5, 1), 2: rw.at(5, 0), 3: rw.at(5, 2)}
	rw.watch(8*time.Second, func(int) bool {
		for row, home := range homes {
			if c, o := rw.state(rw.byRow[row]); c != home || o != nil {
				return false
			}
		}
		return true
	})
	for row, home := range homes {
		if c, o := rw.state(rw.byRow[row]); c != home || o != nil {
			t.Errorf("ally %d stands at %v with %v, want home at %v", row, c, o != nil, home)
		}
	}
}

// Under BodySpacing a stranger standing in the way never gives way: the traveller goes round it;
// an ally gives way and goes back home.
func TestCourtesy_BodiesGoRoundAStrangerAndAnAllyGivesWay(t *testing.T) {
	probe := &fieldWorld{grid: board.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal, home := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80), geom.NewVec(5*fieldCell, 80)
	for _, c := range []struct {
		owner  control.PlayerID
		yields bool
	}{{2, false}, {1, true}} {
		fw := newFieldWorld(t, 10, 5, BodySpacing, nil, []fieldUnit{
			{at: from, side: 6, order: probe.standAt(goal), owner: 1, courtly: true},
			{at: home, side: 6, owner: c.owner, courtly: true},
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
		// struck, it is nudged by the collision while the two talk: home within its side
		if at, o := fw.centre(1); o != nil || math.Hypot(at.X-home.X, at.Y-home.Y) > 6 {
			t.Errorf("player %d's unit stands at %v with order %v, want home at %v", c.owner, at, o != nil, home)
		}
	}
}

// Under BodySpacing two strangers head on pass each other and both arrive.
func TestCourtesy_BodiesOfTwoStrangersHeadOnPass(t *testing.T) {
	probe := &fieldWorld{grid: board.DefaultGrids{}.Square(10, 5, fieldCell)}
	goal0, goal1 := geom.NewVec(9*fieldCell+16, 80), geom.NewVec(16, 80)
	fw := newFieldWorld(t, 10, 5, BodySpacing, nil, []fieldUnit{
		{at: goal1, side: 6, order: probe.standAt(goal0), owner: 1, courtly: true},
		{at: goal0, side: 6, order: probe.standAt(goal1), owner: 2, courtly: true},
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

// A unit's tree orders it as a player would, for itself alone: a patrol along the road, each
// way once it is told Arrived. The player's other unit, selected, is never sent.
func TestTree_APatrolOrdersTheUnitAloneAndGoesOnOnceArrived(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	east, west := rw.at(8, 1), rw.at(1, 1)
	c := act.Named("navigation test patrol")
	patrol := c.Do(c.Then(
		c.Issue(MoveTo{Cell: east}).Until[Arrived](),
		c.Issue(MoveTo{Cell: west}).Until[Arrived]()))
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(0, 1), owner: 1, selected: true, tree: patrol},
		{start: rw.at(0, 0), owner: 1, selected: true},
	})
	walker, bystander := rw.byRow[0], rw.byRow[1]
	var reached []board.CellID
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
