package bench_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

// Benchmark_Board_GroundAt reads the ground at 64 points a quarter cell apart along a diagonal of
// a 256x256 square board of hills 4 cells a side, as sight samples it along one ray.
func Benchmark_Board_GroundAt(b *testing.B) {
	const side, size = 256, 16
	ctx := newHeadless()
	ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: side * size, Height: side * size},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
		Quasi3D:  true,
	})
	grid := board.DefaultGrids{}.Square(side, side, size)
	p := board.NewPlugin(grid, &board.MultipleOccupancy{}, ctx.world)
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	brd := p.Res.Logic.Board
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	brd.SetHeights(board.MeanOfCells(grid, func(c board.CellID) float64 {
		if x, y, _ := grid.Coords(c); (x/4+y/4)%2 == 0 {
			return 12
		}
		return 0
	}))
	ctx.start(b, func(goke.RunCtx, time.Duration) {})

	origin := geom.NewVec(1000.5, 1000.5)
	var sum float64
	for b.Loop() {
		for i := range 64 {
			d := float64(i) * size / 4
			sum += brd.GroundAt(geom.NewVec(origin.X+d*0.8, origin.Y+d*0.6))
		}
	}
	_ = sum
}
