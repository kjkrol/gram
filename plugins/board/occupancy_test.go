package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// The board holds the cell a unit stands on under a raised deck on the level its feet are on: on
// the deck while it stands there, on the ground once it has fallen off it into the same cell.
func TestOccupancy_HoldsAUnitsCellOnTheLevelOfItsFeet(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 3, 10)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 30},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	occupancy := &cell.SingleOccupancy{}
	p := board.NewPlugin(g, occupancy, w)
	p.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	west, _ := grid.Direction(g, -1, 0)
	east, _ := grid.Direction(g, 1, 0)
	decks := board.NewEntities(p, board.Shape{Size: 10}, func(c cell.ID) cell.ID { return c })
	decks.Define("deck", comp.Const(cell.Floor{
		Allows:    cell.Land,
		Height:    10,
		Thickness: 1,
		Links:     1<<west | 1<<east,
		Width:     6,
	}))
	units := board.NewUnits(p, board.Shape{Size: 2, Height: 2},
		func(c cell.ID) geom.Vec { return g.CellCenter(c) })
	units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
	for x := range uint32(3) {
		w.Seed(decks.Named("deck").Entry(g.At(x+1, 1)))
	}
	mid := g.At(2, 1)
	w.Seed(units.Named("walker").Entry(mid))
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var z goke.Comp[world.Z]
	var walker *goke.Query
	ctx.ECS().Setup(append(ctx.Systems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		walker = si.NewQueryBuilder(&z).Build()
	}})...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	standAt := func(alt float64) {
		for walker.All(); walker.Next(); {
			for i := range z.Slice(walker.Cursor()) {
				z.Slice(walker.Cursor())[i].Altitude = alt
			}
		}
		ctx.ECS().Tick(time.Second / 60)
	}
	const other = 99
	standAt(10)
	if occupancy.CanEnter(mid, true, other, cell.Land) ||
		!occupancy.CanEnter(mid, false, other, cell.Land) {
		t.Error("a unit standing on the deck holds its cell's ground, not the deck")
	}
	standAt(0)
	if !occupancy.CanEnter(mid, true, other, cell.Land) ||
		occupancy.CanEnter(mid, false, other, cell.Land) {
		t.Error("a unit fallen off the deck into its cell still holds the deck, not the ground")
	}
}
