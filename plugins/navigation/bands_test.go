package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// two observers looking at (x, y) over a world w by h: the one a pixel a world unit, the other a
// quarter of a pixel seeing twice as far; under a ladder seeing from half a pixel a world unit,
// L0 within its sight of the point, L1 within twice that, L4 beyond
type observers [2]camera.Camera

var _ sector.Observers = (*observers)(nil)

func newObservers(w, h uint32, x, y float32) *observers {
	o := &observers{cameratest.NewFromSpace(w, h, 0), cameratest.NewFromSpace(w, h, 0)}
	for _, cam := range o {
		cam.SetViewport(2, 2)
	}
	o[1].ZoomOut(4, 0, 0)
	o[1].(interface{ See(factor float32) }).See(2)
	o.look(x, y)
	return o
}

// has both observers look at (x, y)
func (o *observers) look(x, y float32) {
	for _, cam := range o {
		cam.CenterOn(x, y, 0)
	}
}

func (o *observers) Observers(each func(cam camera.Camera)) {
	for _, cam := range o {
		each(cam)
	}
}

func TestNavigation_GoesOnAtL1AndStandsAsleep(t *testing.T) {
	const cols, rows = 128, 4
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	profile := steering.Steering{MaxSpeed: float32(legCellSize * 2)}
	walker := func(from, to uint32) navUnit {
		start := g.At(from, 1)
		return navUnit{box: cellBox(g, start, legEntitySize), at: start, profile: &profile,
			order: &MoveOrder{Target: g.At(to, 1)}}
	}
	// at L0, at L1, asleep
	units := []navUnit{walker(2, 8), walker(40, 46), walker(70, 76)}
	var base goke.Comp[world.Base]
	var q *goke.Query
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0.5}, Sight: 600}
	nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units, world.SectorsCfg{Buckets: 2},
		ladder, func(w *world.Plugin, _ *board.Plugin) {
			w.Write().Observe(newObservers(cols*legCellSize, rows*legCellSize, 80, 48))
		},
		goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})
	xs := func() map[int]float32 {
		got := map[int]float32{}
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				for u, of := range nw.ids {
					if of == id {
						got[u] = base.Slice(cur)[i].Pos.Center().X
					}
				}
			}
		}
		return got
	}
	before := xs()
	for range 120 {
		nw.ecs.Tick(time.Second / 60)
	}
	after := xs()
	if cell := float32(legCellSize); after[0] <= before[0]+cell || after[1] <= before[1]+cell {
		t.Errorf("at L0 from %v to %v, at L1 from %v to %v; want both on their way",
			before[0], after[0], before[1], after[1])
	}
	if after[2] != before[2] {
		t.Errorf("asleep from %v to %v, want it standing", before[2], after[2])
	}
}

func TestMoveTo_SendsNoSelectedUnitAsleep(t *testing.T) {
	const cols, rows = 128, 4
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	profile := steering.Steering{MaxSpeed: float32(legCellSize * 2)}
	selected := func(x uint32) navUnit {
		at := g.At(x, 1)
		return navUnit{box: cellBox(g, at, legEntitySize), at: at, profile: &profile,
			selected: true}
	}
	// at L0, asleep
	units := []navUnit{selected(2), selected(70)}
	var order goke.OptComp[MoveOrder]
	var q *goke.Query
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0.5}, Sight: 600}
	nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units, world.SectorsCfg{Buckets: 2},
		ladder, func(w *world.Plugin, _ *board.Plugin) {
			w.Write().Observe(newObservers(cols*legCellSize, rows*legCellSize, 80, 48))
		},
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&goke.Comp[unit.At]{}).Optional(&order).Build()
		}})
	nw.ecs.Tick(time.Second / 60)
	nw.nav.moves.Add(command.Nobody, MoveTo{Cell: g.At(8, 1)})
	nw.ecs.Tick(time.Second / 60)
	ordered := func(k int) bool {
		if !q.Seek(nw.ids[k]) {
			t.Fatalf("unit %d is gone", k)
		}
		return order.At(q.Cursor()) != nil
	}
	if !ordered(0) || ordered(1) {
		t.Errorf("ordered at L0 %v, asleep %v; want only the first", ordered(0), ordered(1))
	}
}
