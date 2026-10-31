package navigation

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/world"
)

var (
	passagesGrid                 = grid.DefaultGrids{}.Square(24, 13, 32)
	passagesFrom, passagesTarget = passagesGrid.At(2, 2), passagesGrid.At(21, 2)
)

// barrier is a wall down the columns of passagesGrid from first to last but in the rows open
type barrier struct {
	first, last uint32
	open        []uint32
}

func (w barrier) lay(brd *board.Plugin, _ *world.Plugin) {
	b := brd.Write().Board()
	for y := range uint32(13) {
		if slices.Contains(w.open, y) {
			continue
		}
		for x := w.first; x <= w.last; x++ {
			b.Set(passagesGrid.At(x, y), cell.Kind{Pace: 1, Solid: true})
		}
	}
}

// crossing ticks ecs for seconds or till the walker, of a box size wide, stands at the target,
// and reports whether it came there and the rows of w's columns its footprint was laid from on
// the way
func crossing(ecs *goke.ECS, at func() geom.Vec, seconds int, size float32,
	w barrier) (arrived bool, rows map[uint32]bool) {
	g, rows := passagesGrid, map[uint32]bool{}
	box := geom.NewVec(size, size)
	goal := unit.Place(g, box, passagesTarget)
	for range 60 * seconds {
		ecs.Tick(time.Second / 60)
		p := at()
		if c, ok := g.CellAt(unit.LaidFrom(g, box, p)); ok {
			if x, y, _ := g.Coords(c); x >= w.first && x <= w.last {
				rows[y] = true
			}
		}
		if f32.Hypot(p.X-goal.X, p.Y-goal.Y) < 4 {
			return true, rows
		}
	}
	return false, rows
}

// a walker of a footprint past a cell walks to its target, collision letting it, the middle of a
// passage or a hole it fits, laid from the rows that fit it, and never into a narrower one, which
// a walker within a cell takes, the nearer; with no way wide enough it never steps into the wall.
// A box exactly three cells wide takes four, the room collision needs to spare.
func TestNavigation_ABodyBiggerThanACellWalksThePassageItFitsAndNoNarrower(t *testing.T) {
	passages := func(open ...uint32) barrier { return barrier{6, 17, open} }
	for _, tc := range []struct {
		name      string
		size      float32
		footprint int // the board's, its default where 0
		wall      barrier
		rows      map[uint32]bool // the rows of the wall the footprint is laid from, crossing
		arrive    bool
	}{
		{"three cells less a pixel", 95, 0, passages(2, 3, 8, 9, 10), map[uint32]bool{9: true},
			true},
		{"two cells and a half", 80, 0, passages(2, 3, 8, 9, 10), map[uint32]bool{9: true},
			true},
		{"within a cell", 22, 0, passages(2, 3, 8, 9, 10), nil, true},
		{"three cells less a pixel, no wide way", 95, 0, passages(2, 3), map[uint32]bool{},
			false},
		{"three cells, by a passage of four", 96, 4, passages(1, 2, 3, 6, 7, 8, 9),
			map[uint32]bool{7: true}, true},
		{"two cells less four pixels, by a passage of two", 60, 0, passages(2, 8, 9),
			map[uint32]bool{8: true}, true},
		{"two cells less four pixels, by a hole of two", 60, 0, barrier{12, 12, []uint32{2, 7, 8}},
			map[uint32]bool{7: true}, true},
		{"two cells less four pixels, no way of two", 60, 0, barrier{12, 12, []uint32{2, 7}},
			map[uint32]bool{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ecs, at, _ := walkerWorld(t, passagesGrid, passagesFrom, passagesTarget,
				tc.wall.lay, limited{size: tc.size},
				func(brd *board.Plugin, _ *Plugin) {
					if tc.footprint > 0 {
						brd.WithFootprint(tc.footprint)
					}
				})
			arrived, rows := crossing(ecs, at, 60, tc.size, tc.wall)
			if arrived != tc.arrive {
				t.Errorf("arrived %v, want %v; stands at %v", arrived, tc.arrive, at())
			}
			if tc.rows == nil {
				if rows[8] || rows[9] || rows[10] || !rows[2] && !rows[3] {
					t.Errorf("crossed in rows %v, want the narrow passage's", rows)
				}
				return
			}
			if !maps.Equal(rows, tc.rows) {
				t.Errorf("its footprint laid from the wall's rows %v, want %v", rows, tc.rows)
			}
		})
	}
}

// a walker of a footprint three cells a side climbs its far route through the hierarchy of its
// domain, the class of its kind's body standing where its footprint does: by the wide passage
func TestNavigation_ABodyBiggerThanACellClimbsByThePassageItFits(t *testing.T) {
	passages := barrier{6, 17, []uint32{2, 3, 8, 9, 10}}
	ecs, at, nav := walkerWorld(t, passagesGrid, passagesFrom, passagesTarget, passages.lay,
		limited{size: 95},
		func(_ *board.Plugin, n *Plugin) { n.WithHierarchy(Hierarchy{Side: 4, From: 8}) })
	arrived, rows := crossing(ecs, at, 60, 95, passages)
	if !arrived || !maps.Equal(rows, map[uint32]bool{9: true}) {
		t.Errorf("arrived %v, crossing in rows %v; want by the wide passage's middle", arrived,
			rows)
	}
	coord := nav.module.navigationSystem.Routing.(*routing.Coordinator)
	if coord.Climbs() == 0 {
		t.Error("no route climbed")
	}
}
