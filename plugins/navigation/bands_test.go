package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

type anchor struct{ x, y float32 }

func (a anchor) Anchors(at func(x, y float32)) { at(a.x, a.y) }

var _ sector.Anchors = anchor{}

func TestNavigation_GoesOnInAFartherBandAndStandsBeyondTheLast(t *testing.T) {
	const cols, rows = 128, 4
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	profile := steering.Steering{MaxSpeed: float32(legCellSize * 2)}
	walker := func(from, to uint32) navUnit {
		start := g.At(from, 1)
		return navUnit{box: cellBox(g, start, legEntitySize), at: start, profile: &profile,
			order: &MoveOrder{Target: g.At(to, 1)}}
	}
	// near, in the second band, beyond the last
	units := []navUnit{walker(2, 8), walker(40, 46), walker(70, 76)}
	var base goke.Comp[world.Base]
	var q *goke.Query
	nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units, world.SectorsCfg{
		Buckets: 2,
		Bands: []sector.Band{
			{Reach: 600},
			{Reach: 1200, Pace: 4},
		},
	}, func(w *world.Plugin, _ *board.Plugin) { w.Write().Anchor(anchor{x: 80, y: 48}) },
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
		t.Errorf("near from %v to %v, in the second band from %v to %v; want both on their way",
			before[0], after[0], before[1], after[1])
	}
	if after[2] != before[2] {
		t.Errorf("beyond the last band from %v to %v, want it standing", before[2], after[2])
	}
}
