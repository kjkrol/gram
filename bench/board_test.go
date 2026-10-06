package bench_test

import (
	"image/color"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/water"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// fixedSky is a topography.Atmosphere of a fixed sun and weather.
type fixedSky struct {
	sun sky.Sun
	air air.Weather
}

func (s fixedSky) Sun() sky.Sun     { return s.sun }
func (s fixedSky) Air() air.Weather { return s.air }

// Benchmark_Board_GroundAt reads the ground at 64 points a quarter cell apart along a diagonal of
// a 256x256 square board of hills 4 cells a side, as sight samples it along one ray.
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
	topo := topography.NewPlugin(ctx.world, p, topography.Config{Cell: size})
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(topo); err != nil {
		b.Fatal(err)
	}
	brd := p.Res.Logic.Board
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	ground := topo.Relief()
	ground.SetHeights(relief.MeanOfCells(grid, func(c cell.ID) float64 {
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
			sum += ground.At(geom.NewVec(origin.X+d*0.8, origin.Y+d*0.6))
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
		Heights:  true,
	})
	grid := grid.DefaultGrids{}.Square(w, h, size)
	p := board.NewPlugin(grid, &cell.MultipleOccupancy{}, ctx.world)
	topo := topography.NewPlugin(ctx.world, p, topography.Config{Cell: size}) // the terrain's shadows are the topography's
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(topo); err != nil {
		b.Fatal(err)
	}
	brd := p.Res.Logic.Board
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	topo.Relief().SetHeights(relief.MeanOfCells(grid, func(c cell.ID) float64 {
		if x, y, _ := grid.Coords(c); (x/4+y/4)%2 == 0 {
			return 20
		}
		return 0
	}))
	atlas := render.NewAtlas()
	atlas.Add(0, 8, render.Solid(color.RGBA{A: 255}))
	atlas.Close()
	p.WithRenderer(atlas)
	ctx.start(b, func(goke.RunCtx, time.Duration) {})
	src := p.Renderer().(render.Source)
	cam := ctx.world.Camera()
	var f render.Frame
	low := fixedSky{sun: sky.Sun{Dir: [3]float32{-0.8, 0.45, 0.3}, Strength: 0.75, Ambient: 0.3}}
	topo.WithAtmosphere(low)
	for _, sc := range []struct {
		name  string
		moved bool
	}{{"shadows=warm", false}, {"shadows=anew", true}} {
		b.Run(sc.name, func(b *testing.B) {
			for b.Loop() {
				if sc.moved {
					low.sun.Dir[0] = -low.sun.Dir[0] // the sun moves: every shadow is stale
					topo.WithAtmosphere(low)
				}
				f.Reset(cam)
				src.Compose(&f, cam)
			}
		})
	}
}

// Benchmark_Board_Shores composes the whole of a 96x64 board of sea round islands 4 cells a side,
// every 8 cells, from above: warm, with the shores as they were, after a cell has changed, every
// shore worked out anew — what a frame pays when the terrain changes — and warm under clouds, the
// weather laid over every tile.
func Benchmark_Board_Shores(b *testing.B) {
	const w, h, size = 96, 64, 32
	ctx := newHeadless()
	ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: w * size, Height: h * size},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
		Heights:  true,
	})
	grid := grid.DefaultGrids{}.Square(w, h, size)
	p := board.NewPlugin(grid, &cell.MultipleOccupancy{}, ctx.world)
	topo := topography.NewPlugin(ctx.world, p, topography.Config{Cell: size}).Style("sea", painter.Style{Shine: 0.9})
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(topo); err != nil {
		b.Fatal(err)
	}
	brd := p.Res.Logic.Board
	sea := cell.Kind{Name: cell.Named("sea"), Cost: 1, Allows: cell.Water}
	brd.SetAll(sea)
	land := cell.Kind{Cost: 1, Allows: cell.Land}
	for y := range uint32(h) {
		for x := range uint32(w) {
			if c, _ := grid.CellIndex(x, y); x%8 < 4 && y%8 < 4 {
				brd.Set(c, land)
			}
		}
	}
	atlas := render.NewAtlas()
	atlas.Add(0, 8, render.Solid(color.RGBA{A: 255}))
	atlas.Close()
	p.WithRenderer(atlas)
	ctx.start(b, func(goke.RunCtx, time.Duration) {})
	src := p.Renderer().(render.Source)
	cam := ctx.world.Camera()
	var f render.Frame
	far, _ := grid.CellIndex(6, 6)
	shallows := sea
	shallows.Cost = 2
	for _, sc := range []struct {
		name    string
		changed bool
		clouds  float32
	}{{"shores=warm", false, 0}, {"shores=anew", true, 0}, {"shores=warm,clouds", false, 0.5}} {
		b.Run(sc.name, func(b *testing.B) {
			topo.WithAtmosphere(fixedSky{sun: sky.DefaultSun, air: air.Weather{Clouds: sc.clouds}})
			for b.Loop() {
				if sc.changed {
					shallows.Cost = 3 - shallows.Cost // the terrain changes: every shore is stale
					brd.Set(far, shallows)
				}
				f.Reset(cam)
				src.Compose(&f, cam)
			}
		})
	}
}

// island seeds a 96x64 board of cells 32 wide with an island in a sea lying Under it: earth, sand
// by the coast and rock on the heights, blending, with the streams and rivers water.Drain works
// out of its heights laid across it as ways, running out to sea; seen from above, isometric or in
// perspective (view "above", "iso" or "persp"); near, a cell 32 pixels across, or far, the whole
// island on a screen of 576 by 384.
func island(b *testing.B, view string, far bool, workers int) (*headless, *board.Board, render.Source) {
	const w, h, size = 96, 64, 32
	ctx := newHeadless()
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: w * size, Height: h * size},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
		Heights:  true,
	}
	if far {
		cfg.Camera.ViewportWidth, cfg.Camera.ViewportHeight = 576, 384
	}
	ctx.UseWorld(cfg)
	grid := grid.DefaultGrids{}.Square(w, h, size)
	p := board.NewPlugin(grid, &cell.MultipleOccupancy{}, ctx.world)
	topo := topography.NewPlugin(ctx.world, p, topography.Config{Cell: size, HeightUnit: 1, Isometric: view != "above", Perspective: view == "persp"})
	kinds := p.CellKinds()
	kinds.Define("sea", cell.Kind{Cost: 1, Allows: cell.Water})
	kinds.Define("earth", cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define("sand", cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define("rock", cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define("stream", cell.Kind{Cost: 2, Allows: cell.Land | cell.Water})
	kinds.Define("estuary", cell.Kind{Cost: 1, Allows: cell.Water})
	topo.Style("sea", painter.Style{Shine: 0.9, Under: true}).
		Style("earth", painter.Style{Spread: 0.3}).
		Style("sand", painter.Style{Spread: 0.35}).
		Style("rock", painter.Style{Spread: 0.25}).
		Style("stream", painter.Style{Shine: 0.9, Flow: 60}).
		Style("estuary", painter.Style{Shine: 0.9, Flow: 45})
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(topo); err != nil {
		b.Fatal(err)
	}
	inside := func(x, y float64) float64 { // 1 in the middle, 0 at the coast, below it at sea
		dx, dy := (x-w/2)/34, (y-h/2)/22
		return 1 - math.Hypot(dx, dy)*(1+0.1*math.Sin(5*math.Atan2(dy, dx)))
	}
	land := map[cell.ID]bool{}
	grid.EachCell(func(c cell.ID) {
		x, y, _ := grid.Coords(c)
		land[c] = inside(float64(x)+0.5, float64(y)+0.5) > 0
	})
	heights := func(q geom.Vec) float64 {
		x, y := q.X/size, q.Y/size
		for _, d := range [4][2]float64{{-0.5, -0.5}, {0.5, -0.5}, {-0.5, 0.5}, {0.5, 0.5}} {
			if c, ok := grid.CellAt(geom.NewVec((x+d[0])*size, (y+d[1])*size)); !ok || !land[c] {
				return 0
			}
		}
		in := inside(x, y)
		return 8 + 160*in*in*(0.7+0.3*math.Sin(x/3)*math.Cos(y/4)) + 10*math.Sin(x/2+y/3)
	}
	rivers, err := water.Drain(grid, heights, func(c cell.ID) bool { return !land[c] }, water.Config{
		BrookAt: 30, StreamAt: 60, RiverAt: 170, BrookDepth: 1, StreamDepth: 2, RiverDepth: 5,
		WidthPerRoot: 1.4, Meander: 6, Plume: 0.25,
	})
	if err != nil {
		b.Fatal(err)
	}
	layout := board.Layout{Default: "sea"}
	topo.Seed(rivers.Carved(heights))
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
	p.Seed(layout)
	atlas := render.NewAtlas()
	for _, k := range kinds.All() {
		atlas.Add(k.SpriteID, 8, render.Solid(color.RGBA{R: 100, G: 150, B: 80, A: 255}))
	}
	atlas.Close()
	p.WithWorkers(workers).WithRenderer(atlas)
	ecs := ctx.start(b, func(ctx goke.RunCtx, d time.Duration) { topo.RunPlan(ctx, d) })
	if view == "persp" { // Tab once, from the isometric view
		for _, q := range topo.Queues() {
			if q.Accepts() == reflect.TypeFor[topography.View]() {
				q.Put(control.Nobody, topography.View{Camera: ctx.world.Camera()})
			}
		}
		ecs.Tick(step)
	}
	if far {
		ctx.world.Camera().ZoomOut(100, w*size/2, h*size/2) // as far as the world fits
	}
	return ctx, p.Res.Logic.Board, p.Renderer().(render.Source)
}

// Benchmark_Board_Island composes the whole of the island, from above and isometric, near and
// far, and the isometric and the perspective island through a 1080p screen, as a player sees it:
// warm, and after a cell ashore has changed — what a frame pays for the ground blending, the coast
// and the running water; the tiles dressed on every CPU at once, and (serial) on one goroutine.
func Benchmark_Board_Island(b *testing.B) {
	for _, v := range []struct {
		view        string
		far, screen bool
		workers     int
	}{{"above", false, false, 0}, {"iso", false, false, 0}, {"iso", false, true, 0}, {"persp", false, true, 0}, {"above", true, false, 0}, {"iso", true, false, 0},
		{"above", false, false, 1}, {"iso", false, false, 1}, {"iso", false, true, 1}, {"persp", false, true, 1}, {"above", true, false, 1}, {"iso", true, false, 1}} {
		ctx, brd, src := island(b, v.view, v.far, v.workers)
		cam := ctx.world.Camera()
		if v.screen {
			cam.SetViewport(1920, 1080)
			cam.CenterOn(96*32/2, 64*32/2, 0)
		}
		far, _ := brd.CellIndex(48, 32)
		rock, _ := brd.CellIndex(48, 30)
		view := v.view
		if v.far {
			view += ",far"
		}
		if v.screen {
			view += ",screen"
		}
		if v.workers == 1 {
			view += ",serial"
		}
		var f render.Frame
		for _, changed := range []bool{false, true} {
			name := view + ",warm"
			if changed {
				name = view + ",changed"
			}
			b.Run(name, func(b *testing.B) {
				for i := 0; b.Loop(); i++ {
					if changed {
						if i%2 == 0 {
							brd.Set(far, brd.Kind(rock))
						} else {
							brd.Set(far, brd.Kind(far))
						}
					}
					f.Reset(cam)
					src.Compose(&f, cam)
				}
			})
		}
	}
}
