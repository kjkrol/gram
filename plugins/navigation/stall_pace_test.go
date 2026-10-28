package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
)

func TestBodySpacing_AStallIsJudgedByTheLawfulPace(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 1, 32)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	finder := spacing.NewFinder(grid, terrain, nil, spacing.OpenOccupancy{})
	from, toward := grid.At(0, 0), grid.At(1, 0)
	start, want := grid.CellCenter(from), grid.CellCenter(toward)
	cases := []struct {
		name    string
		share   float32
		speed   float32 // world units a second
		stalled bool
	}{
		{"slow ground, a crawl over a quarter of the lawful pace", 0.049, 0.5, false},
		{"open ground, the same crawl", 1, 0.5, true},
		{"a step the ground refuses", 0, 0, false},
		{"slow ground, under a quarter of the lawful pace", 0.049, 0.1, true},
	}
	for _, c := range cases {
		keep := spacing.NewBodyKeeping(finder, nil, nil)
		keep.Begin(func(dst []bodies.Body) []bodies.Body { return dst })
		m := bodies.Member{ID: 7, Cell: from, From: from, Domain: cell.Land, MaxSpeed: 24,
			Share: c.share}
		var o MoveOrder
		bumped := false
		for tick := range 120 {
			x := start.X + c.speed*float32(tick)/60
			m.Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(x-1.44, start.Y-1.44), 2.88, 2.88)}
			keep.Watch(m, &o, toward, want, time.Second/60)
			bumped = bumped || o.Bumped
		}
		if bumped != c.stalled {
			t.Errorf("%s: bumped %v after two seconds, want %v", c.name, bumped, c.stalled)
		}
	}
}

func TestBodySpacing_ACrawlOverSlowGroundIsNoStall(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(7, 1, fieldCell)}
	strip := func(b testBoard, at func(x, y uint32) cell.ID) {
		for _, x := range []uint32{3, 4} {
			b.Set(at(x, 0), cell.Kind{Pace: 0.01, Allows: cell.Land})
		}
	}
	from, goal := probe.grid.CellCenter(probe.at(0, 0)), probe.grid.CellCenter(probe.at(6, 0))
	fw := newFieldWorld(t, 7, 1, BodySpacing, strip,
		[]fieldUnit{{at: from, side: 6, paced: true, order: probe.standAt(goal)}})
	for range 12 * 60 {
		fw.ecs.Tick(time.Second / 60)
	}
	at, o := fw.centre(0)
	if onStrip := at.X >= 3*fieldCell && at.X < 5*fieldCell; o == nil || o.Stalls > 0 || !onStrip {
		t.Fatalf("after 12 s at %v: order %v, want it still crawling over the strip without a stall",
			at, o)
	}
	settled, _ := fw.run(120 * time.Second)
	if at, _ := fw.centre(0); !settled || f32.Hypot(at.X-goal.X, at.Y-goal.Y) > 1e-3 {
		t.Errorf("the unit stands at %v, settled %v, want it across the strip at %v", at, settled, goal)
	}
}

func TestNavigation_LeadsTheNextCellAndNoneWhenItStands(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 1, legCellSize)
	start, target := grid.At(0, 0), grid.At(3, 0)
	ew := newEnteredWorld(t, 6, 1, start, target, true)
	led := false
	for range 600 {
		ew.ecs.Tick(time.Second / 60)
		at := ew.ats[ew.id]
		if ew.hasOrder[ew.id] {
			led = led || at.Cell == start && at.Next == grid.At(1, 0)
			continue
		}
		if !led {
			t.Error("never led into (1,0) while it stood in (0,0)")
		}
		if at.Next != 0 {
			t.Errorf("arrived in %v, led into %v, want nowhere", at.Cell, at.Next)
		}
		return
	}
	t.Fatal("never arrived")
}
