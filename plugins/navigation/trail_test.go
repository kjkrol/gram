package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
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

func TestCellSpacing_AQueueBehindACrawlingGroupmateWaitsItsLawfulTimeThenGoesRound(t *testing.T) {
	rw := newRoadWorld(t, 10, []roadUnit{{start: 0, ordered: true}})
	rw = newRoadWorld(t, 10, []roadUnit{
		{start: rw.at(2, 1), target: rw.at(9, 1), ordered: true, group: 1, paced: true},
		{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true, group: 1, paced: true},
	})
	leader, tail := rw.byRow[0], rw.byRow[1]
	rw.ecs.Tick(time.Second / 60)
	// slowed once both routes run over it: a route planned after goes round
	rw.nav.boardPlugin.Write().Board().Set(rw.at(3, 1),
		cell.Kind{Pace: 0.01, Allows: cell.Land | cell.Air})
	var trailed, stalled time.Duration // since the start, when the tail first did
	offRoad, arrived := false, false
	for tick := 2; tick <= 30*60; tick++ {
		rw.ecs.Tick(time.Second / 60)
		now := time.Duration(tick) * (time.Second / 60)
		c, o := rw.state(tail)
		if o == nil {
			arrived = true
			break
		}
		if trailed == 0 && o.TrailFor > 0 && o.Trailing == leader {
			trailed = now
		}
		if stalled == 0 && o.Stalls > 0 {
			stalled = now
			if !slices.Contains(o.Avoid[:o.Avoids], rw.at(2, 1)) {
				t.Errorf("stalled, the tail goes round %v, want the cell the leader holds, (2,1)",
					o.Avoid[:o.Avoids])
			}
		}
		_, row, _ := rw.grid.Coords(c)
		offRoad = offRoad || row != 1
	}
	if trailed == 0 || trailed > time.Second {
		t.Fatalf("the tail trailed the leader from %v, want within the first second", trailed)
	}
	if stalled == 0 || stalled-trailed < 3*time.Second {
		t.Errorf("the tail stalled %v after it began trailing, want after a wait of 3 s",
			stalled-trailed)
	}
	c, _ := rw.state(tail)
	_, led := rw.state(leader)
	if !arrived || !offRoad || led == nil || rw.grid.Distance(c, rw.at(9, 1)) > 1.5 {
		t.Errorf("the tail stands at %v, arrived %v, off the road %v, the leader under way %v; "+
			"want it round the leader and by the goal first", c, arrived, offRoad, led != nil)
	}
}
