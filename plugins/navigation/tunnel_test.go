package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

// a ridge 30 high from north to south across the board, a tunnel through it at the ground's
// level either side
func ridgeTunnel(clear float32) bridgeSite {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	return bridgeSite{
		heights: func(p geom.Vec) float32 {
			if p.X > 64 && p.X < 160 {
				return 30
			}
			return 0
		},
		from: g.At(1, 3),
		to:   g.At(5, 3),
		floor: func(level float32) cell.Floor {
			return cell.Floor{
				Allows: cell.Land,
				Height: -level,
				Clear:  clear,
				Pace:   1,
				Graded: true,
				Width:  12,
				Storey: 1,
			}
		},
	}
}

func TestNavigation_WalksThroughATunnelUnderARidge(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	var z goke.Comp[world.Z]
	var base goke.Comp[world.Base]
	var q *goke.Query
	ecs, _, _, _ := bridged(t, ridgeTunnel(4), geom.NewVec(16, 112), g.At(7, 3), EitherLevel,
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&base, &z).Build()
		}})
	under, highest, at := false, float32(0), geom.Vec{}
	for range 60 * 30 {
		ecs.Tick(time.Second / 60)
		for q.All(); q.Next(); {
			for i, b := range base.Slice(q.Cursor()) {
				if b.Pos.Size.X != 3 {
					continue
				}
				at = b.Pos.Center()
				alt := z.Slice(q.Cursor())[i].Altitude
				highest = max(highest, alt)
				if c, _ := g.CellAt(at); c == g.At(3, 3) && alt < 1 {
					under = true
				}
			}
		}
	}
	if c, _ := g.CellAt(at); c != g.At(7, 3) {
		t.Fatalf("after 30 s the walker stands at %v, want past the ridge", at)
	}
	if !under || highest > 2 {
		t.Errorf("through the ridge's middle under it %v, as high as %.1f on the way; want "+
			"the tunnel's floor all the way", under, highest)
	}
}

func TestNavigation_ATunnelTooLowSendsTheTallOverTheRidge(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	var z goke.Comp[world.Z]
	var base goke.Comp[world.Base]
	var q *goke.Query
	ecs, _, _, _ := bridged(t, ridgeTunnel(2), geom.NewVec(16, 112), g.At(7, 3), EitherLevel,
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&base, &z).Build()
		}})
	highest, at := float32(0), geom.Vec{}
	for range 60 * 30 {
		ecs.Tick(time.Second / 60)
		for q.All(); q.Next(); {
			for i, b := range base.Slice(q.Cursor()) {
				if b.Pos.Size.X != 3 {
					continue
				}
				at = b.Pos.Center()
				alt := z.Slice(q.Cursor())[i].Altitude
				highest = max(highest, alt)
				if at.X > 100 && at.X < 124 && alt < 25 {
					t.Fatalf("a walker 3 tall stands %.1f up at %v, in a tunnel 2 high", alt, at)
				}
			}
		}
	}
	if c, _ := g.CellAt(at); c != g.At(7, 3) || highest < 29 {
		t.Errorf("after 30 s the walker stands at %v, as high as %.1f on the way; want over "+
			"the ridge and past it", at, highest)
	}
}
