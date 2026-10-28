package bench_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/board/water"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
)

// Benchmark_Board_GroundAt reads the ground at 64 points along a diagonal of a 256x256 board of
// hills, as sight samples it along one ray.
func Benchmark_Board_GroundAt(b *testing.B) {
	const side, size = 256, 16
	ctx := newHeadless()
	ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: side * size, Height: side * size},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
		Heights:  true,
	})
	grid := grid.DefaultGrids{}.Square(side, side, size)
	p := board.NewPlugin(grid, &cell.MultipleOccupancy{}, ctx.world)
	topo := topography.NewPlugin(ctx.world, p, nil, topography.Config{})
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(topo); err != nil {
		b.Fatal(err)
	}
	brdW := p.Write().Board()
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	relief := topo.Write().Relief()
	relief.SetHeights(ground.MeanOfCells(grid, func(c cell.ID) float32 {
		if x, y, _ := grid.Coords(c); (x/4+y/4)%2 == 0 {
			return 12
		}
		return 0
	}))
	ctx.start(b, func(goke.RunCtx, time.Duration) {})

	origin := geom.NewVec(1000.5, 1000.5)
	var sum float32
	for b.Loop() {
		for i := range 64 {
			d := float32(i) * size / 4
			sum += relief.At(geom.NewVec(origin.X+d*0.8, origin.Y+d*0.6))
		}
	}
	_ = sum
}

// view is "above", "iso" or "persp"; far shows the whole island on a screen of 576 by 384
func island(b *testing.B, view string, far bool) (*headless, *board.Plugin, render.Source) {
	ctx, brd, src, _ := islandWith(b, view, far, true, false, false)
	return ctx, brd, src
}

// islandWith is island with the water's shine or without it (no water layer drawn), under a
// distant sun with the ground's shade baked or in no light at all, and cut into sectors of 256
// or whole; the topography is handed back for its care
func islandWith(b *testing.B, view string, far bool, shine, lit, sectored bool) (
	*headless, *board.Plugin, render.Source, *topography.Plugin) {
	const w, h, size = 96, 64, 32
	ctx := newHeadless()
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: w * size, Height: h * size},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
		Heights:  true,
	}
	if sectored {
		// sectors of 256, every one in the band
		cfg.Sectors = world.SectorsCfg{Buckets: 1, Bands: []sector.Band{{Reach: w * size}}}
	}
	ctx.UseWorld(cfg)
	grid := grid.DefaultGrids{}.Square(w, h, size)
	p := board.NewPlugin(grid, &cell.MultipleOccupancy{}, ctx.world)
	var l *light.Plugin
	if lit {
		sun := light.Distant{Dir: [3]float32{-0.8, 0.45, 0.3}, Strength: 0.75, Ambient: 0.3}
		l = light.NewPlugin(ctx.world, light.Config{}).WithDistant(sun)
	}
	topo := topography.NewPlugin(ctx.world, p, l, topography.Config{})
	var screenW, screenH float32
	if far {
		screenW, screenH = 576, 384
	}
	start := cameras.Isometrically
	if view == "above" {
		start = cameras.FromAbove
	}
	ctx.cam = ctx.camera(cameras.Config{
		Turn:        true,
		Tilt:        true,
		Perspective: view == "persp",
		Cell:        size,
		HeightUnit:  1,
		Cage:        topo.Read(0),
	}, start, screenW, screenH)
	kinds := p.Read(0).CellKinds()
	kinds.Define("sea", cell.Kind{Cost: 1, Allows: cell.Water})
	kinds.Define("earth", cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define("sand", cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define("rock", cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define("stream", cell.Kind{Cost: 2, Allows: cell.Land | cell.Water})
	kinds.Define("estuary", cell.Kind{Cost: 1, Allows: cell.Water})
	var wet float32
	if shine {
		wet = 0.9
	}
	topo.Style("sea", painter.Style{Shine: wet, Under: true}).
		Style("earth", painter.Style{Spread: 0.3}).
		Style("sand", painter.Style{Spread: 0.35}).
		Style("rock", painter.Style{Spread: 0.25}).
		Style("stream", painter.Style{Shine: wet, Flow: 60}).
		Style("estuary", painter.Style{Shine: wet, Flow: 45})
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	if l != nil {
		if err := ctx.Use(l); err != nil {
			b.Fatal(err)
		}
	}
	if err := ctx.Use(topo); err != nil {
		b.Fatal(err)
	}
	inside := func(x, y float32) float32 { // 1 in the middle, 0 at the coast
		dx, dy := (x-w/2)/34, (y-h/2)/22
		return 1 - f32.Hypot(dx, dy)*(1+0.1*f32.Sin(5*f32.Atan2(dy, dx)))
	}
	land := map[cell.ID]bool{}
	grid.EachCell(func(c cell.ID) {
		x, y, _ := grid.Coords(c)
		land[c] = inside(float32(x)+0.5, float32(y)+0.5) > 0
	})
	heights := func(q geom.Vec) float32 {
		x, y := q.X/size, q.Y/size
		for _, d := range [4][2]float32{{-0.5, -0.5}, {0.5, -0.5}, {-0.5, 0.5}, {0.5, 0.5}} {
			if c, ok := grid.CellAt(geom.NewVec((x+d[0])*size, (y+d[1])*size)); !ok || !land[c] {
				return 0
			}
		}
		in := inside(x, y)
		return 8 + 160*in*in*(0.7+0.3*f32.Sin(x/3)*f32.Cos(y/4)) + 10*f32.Sin(x/2+y/3)
	}
	rivers, err := water.Drain(grid, heights, func(c cell.ID) bool { return !land[c] }, water.Config{
		BrookAt: 30, StreamAt: 60, RiverAt: 170, BrookDepth: 1, StreamDepth: 2, RiverDepth: 5,
		WidthPerRoot: 1.4, Meander: 6, Plume: 0.25,
	})
	if err != nil {
		b.Fatal(err)
	}
	layout := board.Layout{Default: "sea"}
	topo.Write().Seed(rivers.Carved(heights))
	grid.EachCell(func(c cell.ID) {
		if land[c] {
			at := grid.CellCenter(c)
			kind := "earth"
			switch in := inside(at.X/size, at.Y/size); {
			case in < 0.08:
				kind = "sand"
			case heights(at) > 110:
				kind = "rock"
			}
			layout.Cells = append(layout.Cells, cell.Entry{Kind: kind, Cell: c})
		}
	})
	layout.Ways = rivers.Net(map[water.Course]string{water.Brook: "stream", water.Stream: "stream", water.River: "stream",
		water.Ford: "stream", water.Mouth: "estuary"}).Ways()
	p.Write().Seed(layout)
	atlas := render.NewAtlas()
	for _, k := range kinds.All() {
		atlas.Add(k.SpriteID, 8, render.Solid(color.RGBA{R: 100, G: 150, B: 80, A: 255}))
	}
	atlas.Close()
	p.Write().SetAtlas(atlas)
	ctx.look()
	ctx.pending = append(ctx.pending, ctx.bound)
	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		ctx.world.RunPlan(rc, d)
		if l != nil {
			l.RunPlan(rc, d)
		}
		topo.RunPlan(rc, d)
	})
	if view == "persp" {
		cameratest.Switch(ctx.cam)
		ecs.Tick(step)
	}
	if far {
		ctx.cam.ZoomOut(100, w*size/2, h*size/2)
	}
	return ctx, p, drawnAs[*look.Renderer](b, ctx), topo
}
