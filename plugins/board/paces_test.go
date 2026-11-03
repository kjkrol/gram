package board_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// the world's one observer, a camera from above looking at (x, y)
type eye struct{ cam camera.Camera }

var _ sector.Observers = eye{}

func lookingAt(w, h uint32, x, y float32) eye {
	cam := cameratest.NewFromSpace(w, h, 0)
	cam.SetViewport(2, 2)
	cam.CenterOn(x, y, 0)
	return eye{cam}
}

func (e eye) Observers(each func(cam camera.Camera)) { each(e.cam) }

func TestBoard_AUnitAsleepAtL4StandsForNoRuleAndHoldsNoCellItIsPutIn(t *testing.T) {
	const cols, rows = 40, 2
	g := grid.DefaultGrids{}.Square(cols, rows, boardtest.CellSize)
	near, far := g.At(1, 0), g.At(38, 0)
	var iced effect.Effect
	var base goke.Comp[world.Base]
	var at goke.Comp[unit.At]
	var q *goke.Query
	w, h := uint32(cols*boardtest.CellSize), uint32(rows*boardtest.CellSize)
	bw := boardtest.NewWorldIn(t, g, w, h, func(cfg *world.Config) {
		cfg.Sectors = world.SectorsCfg{Buckets: 1}
		cfg.LOD = lod.Ladder{Sight: 300}
	}, func(wp *world.Plugin, brd *board.Plugin) []act.Rule {
		c := g.CellCenter(near)
		wp.Write().Observe(lookingAt(w, h, c.X, c.Y))
		brd.Write().Board().SetAll(cell.Kind{Name: cell.Named("grass"), Pace: 1,
			Allows: cell.Land})
		wp.Write().Effects().Define("iced", effect.Spec{})
		iced = wp.Write().Effects().Named("iced")
		return []act.Rule{act.Branch[unit.Standing]{
			Name: "freeze",
			When: cond.Self.If(cond.Always),
			Then: act.Branch[any]{
				When: cond.At(board.Around(0)).If(cond.Not(iced)),
				Then: act.At(board.Around(0)).Put(iced),
			},
		}}
	}, []boardtest.Mover{{Here: near}, {Here: far}},
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&base, &at).Build()
		}})
	for range 4 {
		bw.Tick()
	}
	brd := bw.Board.Read(0).Board()
	if !brd.States(near).Has(iced.Mark()) || brd.States(far).Has(iced.Mark()) {
		t.Errorf("iced under the unit at L0 %v, asleep %v; want only the first",
			brd.States(near).Has(iced.Mark()), brd.States(far).Has(iced.Mark()))
	}
	east := geom.NewVec(boardtest.CellSize, 0)
	for q.All(); q.Next(); {
		for i := range base.Slice(q.Cursor()) {
			b := &base.Slice(q.Cursor())[i]
			b.Pos.AABB = plane.NewAABB(b.Pos.TopLeft.Add(east), b.Pos.Size.X, b.Pos.Size.Y)
		}
	}
	bw.Tick()
	held := map[cell.ID]bool{}
	for q.All(); q.Next(); {
		for _, a := range at.Slice(q.Cursor()) {
			held[a.Cell] = true
		}
	}
	if !held[g.At(2, 0)] || !held[far] {
		t.Errorf("put a cell east, the units are at %v, want %v and, asleep, still %v", held,
			g.At(2, 0), far)
	}
}
