package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestBodySpacing_TrailingAsksNoMoreThanTheTrailedOneMakes(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 1, 32)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	keep := spacing.NewBodyKeeping(spacing.NewFinder(grid, terrain, nil, spacing.OpenOccupancy{}),
		nil, nil)
	east := geom.NewVec(1, 0)
	led := bodies.Body{ID: 3, At: grid.CellCenter(grid.At(2, 0)), Half: geom.NewVec(3, 3),
		Vel: geom.NewVec(0.96, 0), Facing: east, Domain: cell.Land, Moving: true}
	keep.Begin(func(dst []bodies.Body) []bodies.Body { return append(dst, led) })
	for _, c := range []struct {
		name  string
		share float32
		dir   geom.Vec
		whom  uid.UID64
		want  float32
	}{
		{"on ground leaving it half its pace", 0.5, east, led.ID, 1.92},
		{"on open ground", 1, east, led.ID, 0.96},
		{"heading across their way", 1, geom.NewVec(0, -1), led.ID, 0},
		{"the one trailed gone from the tick", 1, east, 9, 96},
	} {
		o := MoveOrder{Trailing: c.whom, TrailFor: spacing.TrailMemory}
		m := bodies.Member{ID: 7, Cell: grid.At(1, 0), From: grid.At(1, 0), Domain: cell.Land,
			Pos: posAt(grid, grid.At(1, 0)), Facing: east, Share: c.share, MaxSpeed: 96, Order: &o}
		helm := steering.Helm{Steering: &steering.Steering{MaxSpeed: 96},
			Course: &steering.Course{}}
		keep.Steer(m, helm, c.dir, 96)
		if f32.Abs(helm.WantSpeed-c.want) > 1e-5 {
			t.Errorf("%s: asked for %v, want %v", c.name, helm.WantSpeed, c.want)
		}
	}
}

// three in a row on their way east over a strip of slow ground, the leader first, the last of
// group last and the others of group 1
func newColumn(t *testing.T, last uint32) (*fieldWorld, geom.Vec) {
	t.Helper()
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(7, 1, fieldCell)}
	strip := func(b testBoard, at func(x, y uint32) cell.ID) {
		for _, x := range []uint32{3, 4} {
			b.Set(at(x, 0), cell.Kind{Pace: 0.01, Allows: cell.Land})
		}
	}
	goal := probe.grid.CellCenter(probe.at(6, 0))
	units := make([]fieldUnit, 3)
	for i, group := range []uint32{1, 1, last} {
		o := probe.standAt(goal)
		o.Group = group
		units[i] = fieldUnit{at: geom.NewVec(44-14*float32(i), 16), side: 6, paced: true, order: o}
	}
	return newFieldWorld(t, 7, 1, BodySpacing, strip, units), goal
}

func TestCrowd_AColumnOfGroupmatesTrailsItsLeaderOverSlowGround(t *testing.T) {
	fw, goal := newColumn(t, 1)
	var trailed [3]bool // each the one ahead of it, early on the strip
	busy := true
	for tick := 1; busy && tick <= 120*60; tick++ {
		fw.ecs.Tick(time.Second / 60)
		now := time.Duration(tick) * (time.Second / 60)
		busy = false
		for i := range 3 {
			_, o := fw.centre(i)
			if o == nil {
				if now < 12*time.Second {
					t.Fatalf("%v: unit %d has no order, want the column crawling over the strip",
						now, i)
				}
				continue
			}
			busy = true
			if o.Stalls > 0 {
				t.Fatalf("%v: unit %d stalled, want none in a column at its leader's pace", now, i)
			}
			if i > 0 && now < 3*time.Second && o.TrailFor > 0 && o.Trailing == fw.ids[i-1] {
				trailed[i] = true
			}
		}
	}
	if !trailed[1] || !trailed[2] {
		t.Errorf("trailed the one ahead early on the strip: second %v, third %v, want both",
			trailed[1], trailed[2])
	}
	for i := range 3 {
		at, o := fw.centre(i)
		if o != nil || f32.Hypot(at.X-goal.X, at.Y-goal.Y) > 3*6*(1+spacing.SpacingGap) {
			t.Errorf("unit %d stands at %v, under an order %v, want it arrived by %v", i, at,
				o != nil, goal)
		}
	}
}

func TestCrowd_AnAllyOfAnotherGroupIsNotTrailed(t *testing.T) {
	fw, _ := newColumn(t, 2)
	struck := false
	for range 30 * 60 {
		fw.ecs.Tick(time.Second / 60)
		fw.each(func(id uid.UID64, _ *world.Base, _ cell.ID, o *MoveOrder, c *collision.Collider) {
			if id != fw.ids[2] {
				return
			}
			if o != nil && (o.TrailFor > 0 || o.Trailing != 0) {
				t.Fatalf("the last, of another group, trails %d, want it trailing nobody",
					o.Trailing)
			}
			for _, k := range c.Contacts() {
				struck = struck || k.Other == fw.ids[1] && !k.Resting
			}
		})
	}
	if !struck {
		t.Error("the last never struck the one ahead of it, want it met and gone round or stalled")
	}
}

func TestCrowd_AQueueAtADeadEndUnwinds(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(8, 3, fieldCell)}
	deadEnd := func(b testBoard, at func(x, y uint32) cell.ID) {
		for x := uint32(0); x < 8; x++ {
			b.Set(at(x, 0), wall)
			b.Set(at(x, 2), wall)
		}
		b.Set(at(7, 1), wall)
	}
	units := []fieldUnit{{at: geom.NewVec(150, 48), side: 20}} // standing with no order
	for i := range 3 {
		o := probe.standAt(geom.NewVec(210, 48))
		o.Group = 1
		units = append(units, fieldUnit{at: geom.NewVec(100-28*float32(i), 48), side: 20, order: o})
	}
	fw := newFieldWorld(t, 8, 3, BodySpacing, deadEnd, units)
	var trailed [4]bool
	for range 15 * 60 {
		fw.ecs.Tick(time.Second / 60)
		for i := 1; i <= 3; i++ {
			if _, o := fw.centre(i); o != nil {
				trailed[i] = trailed[i] || o.TrailFor > 0 && o.Trailing == fw.ids[i-1]
			}
		}
	}
	for i := 1; i <= 3; i++ {
		if at, o := fw.centre(i); o != nil {
			t.Errorf("after 15 s unit %d at %v still has its order, trailing %d for %v: want the "+
				"queue's orders gone", i, at, o.Trailing, o.TrailFor)
		}
	}
	if !trailed[2] || !trailed[3] {
		t.Errorf("trailed the one ahead: second %v, third %v; want both before the queue stood",
			trailed[2], trailed[3])
	}
}

// the road's one cell slow, the leader on it and the tail behind on their way to (9,1)
func newSlowRoad(t *testing.T, pace float64, only bool) (rw *roadWorld, leader, tail uid.UID64) {
	t.Helper()
	rw = newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(2, 1), target: rw.at(9, 1), ordered: true, group: 1, paced: true},
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, group: 1, paced: true},
	})
	slow := cell.Kind{Pace: pace, Allows: cell.Land | cell.Air}
	if only {
		rw.walls(3, 0, 3, 2)
		rw.nav.boardPlugin.Write().Board().Set(rw.at(3, 1), slow)
		return rw, rw.byRow[0], rw.byRow[1]
	}
	rw.ecs.Tick(time.Second / 60)
	// slowed once both routes run over it: a route planned after goes round
	rw.nav.boardPlugin.Write().Board().Set(rw.at(3, 1), slow)
	return rw, rw.byRow[0], rw.byRow[1]
}

func TestCellSpacing_AQueueBehindACrawlingGroupmateFollowsIt(t *testing.T) {
	rw, leader, tail := newSlowRoad(t, 0.02, true)
	var led, longest time.Duration // when the leader arrived; the tail's longest wait
	trailed := false
	for tick := 1; tick <= 60*60; tick++ {
		rw.ecs.Tick(time.Second / 60)
		now := time.Duration(tick) * (time.Second / 60)
		if _, lo := rw.state(leader); lo == nil && led == 0 {
			led = now
		}
		c, o := rw.state(tail)
		if o == nil {
			if led == 0 || rw.grid.Distance(c, rw.at(9, 1)) > 1.5 {
				t.Errorf("the tail arrived at %v, the leader %v before; want it by the goal after "+
					"the leader", c, now-led)
			}
			if !trailed || longest <= 3*time.Second {
				t.Errorf("the tail trailed the leader %v, its longest wait %v; want it waiting on "+
					"the crawl past the old bound of 3 s", trailed, longest)
			}
			return
		}
		if o.Stalls > 0 {
			t.Fatalf("%v: the tail stalled at %v behind its crawling leader", now, c)
		}
		trailed = trailed || o.TrailFor > 0 && o.Trailing == leader
		longest = max(longest, o.Trailed)
	}
	t.Fatal("the tail never arrived")
}

func TestCellSpacing_AQueueBehindAGroupmateStuckGoesRound(t *testing.T) {
	rw, leader, tail := newSlowRoad(t, 0.01, false)
	halted := rw.w.Write().Effects().Named("halted")
	const halt = 2 * 60 // the tick the leader is halted at, its order kept
	haltAt := time.Duration(halt) * (time.Second / 60)
	var trailedTill time.Duration // the last moment the tail trailed the leader
	for tick := 2; tick <= 20*60; tick++ {
		now := time.Duration(tick) * (time.Second / 60)
		if tick == halt {
			rw.w.Write().Carrier().Put(1, ceremony.Put(halted).On(ceremony.ID(leader)))
		}
		rw.ecs.Tick(time.Second / 60)
		c, o := rw.state(tail)
		if o != nil {
			if o.TrailFor > 0 && o.Trailing == leader {
				trailedTill = now
			}
			continue
		}
		if trailedTill < time.Second || trailedTill > haltAt+spacing.TrailMemory+time.Second/10 {
			t.Errorf("the tail trailed the leader till %v, halted at %v; want its trail lapsed "+
				"within %v", trailedTill, haltAt, spacing.TrailMemory)
		}
		if _, lo := rw.state(leader); c != rw.at(9, 1) || now-haltAt > 10*time.Second || lo == nil {
			t.Errorf("%v after the halt the tail arrived at %v, the leader under its order %v; "+
				"want it round the stuck leader to (9,1) within 10 s", now-haltAt, c, lo != nil)
		}
		return
	}
	t.Fatal("the tail behind the stuck leader never arrived")
}

func TestCrowd_TheGroupIsJoinedAtItsGoalNotOnTheWay(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(34, 3, fieldCell)}
	goal := probe.grid.CellCenter(probe.at(33, 1))
	for _, c := range []struct {
		name   string
		x      uint32 // the column the groupmate stands in, its order over
		joined bool
	}{{"30 cells short of the goal", 3, false}, {"beside the goal", 32, true}} {
		ordered := func(to geom.Vec) *MoveOrder {
			o := probe.standAt(to)
			o.Group = 1
			return o
		}
		standing := probe.grid.CellCenter(probe.at(c.x, 1))
		fw := newFieldWorld(t, 34, 3, BodySpacing, nil, []fieldUnit{
			{at: probe.grid.CellCenter(probe.at(0, 1)), side: 6, order: ordered(goal)},
			{at: standing, side: 6, order: ordered(standing)}, // arrives at once
		})
		if settled, _ := fw.run(30 * time.Second); !settled {
			t.Fatalf("%s: the traveller has not settled", c.name)
		}
		at, _ := fw.centre(0)
		if atGoal := f32.Hypot(at.X-goal.X, at.Y-goal.Y) < 1; atGoal == c.joined {
			t.Errorf("%s: the traveller stands at %v, at its goal %v; want it joined the group %v",
				c.name, at, atGoal, c.joined)
		}
	}
}
