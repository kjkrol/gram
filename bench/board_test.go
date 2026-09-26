package bench_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
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

// Benchmark_Board_Shadows composes the whole of a 96x64 board of hills 4 cells a side, 20 high,
// from above: warm, with the shadows as they were, and after the sun has moved, every shadow
// worked out anew — what a frame pays when the terrain or the sun changes.
func Benchmark_Board_Shadows(b *testing.B) {
	const w, h, size = 96, 64, 32
	ctx := newHeadless()
	ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: w * size, Height: h * size},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
		Quasi3D:  true,
	})
	grid := board.DefaultGrids{}.Square(w, h, size)
	p := board.NewPlugin(grid, &board.MultipleOccupancy{}, ctx.world)
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	brd := p.Res.Logic.Board
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	brd.SetHeights(board.MeanOfCells(grid, func(c board.CellID) float64 {
		if x, y, _ := grid.Coords(c); (x/4+y/4)%2 == 0 {
			return 20
		}
		return 0
	}))
	atlas := render.NewAtlas()
	atlas.RegisterAt(0, 8, render.Solid(color.RGBA{A: 255}))
	atlas.Close()
	p.WithRenderer(atlas)
	ctx.start(b, func(goke.RunCtx, time.Duration) {})
	src := p.Renderer().(render.Source)
	cam := ctx.world.Camera()
	var f render.Frame
	low := world.Sun{Dir: [3]float32{-0.8, 0.45, 0.3}, Strength: 0.75, Ambient: 0.3}
	for _, sc := range []struct {
		name  string
		moved bool
	}{{"shadows=warm", false}, {"shadows=anew", true}} {
		b.Run(sc.name, func(b *testing.B) {
			for b.Loop() {
				if sc.moved {
					low.Dir[0] = -low.Dir[0] // the sun moves: every shadow is stale
					ctx.world.SetSun(low)
				}
				f.Reset(cam)
				src.Compose(&f, cam)
			}
		})
	}
}
