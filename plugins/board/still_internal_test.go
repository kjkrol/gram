package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// A flat board under the simple map, seen from above, is composed once and drawn by the GPU as the
// tiles composed every frame are — kinds, ways and all, zoomed in and moved — composed anew when a
// cell changes, and drawn on across a wrapping world's seam.
func TestRenderer_DrawsAFlatBoardComposedOnceAsEveryFrame(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land, Color: color.RGBA{R: 60, G: 160, B: 60, A: 255}}
	water := cell.Kind{SpriteID: 2, Cost: 1, Allows: cell.Water, Color: color.RGBA{R: 40, G: 80, B: 200, A: 255}}
	road := cell.Kind{Cost: 1, Allows: cell.Land, Color: color.RGBA{R: 150, G: 120, B: 80, A: 255}}
	grid := DefaultGrids{}.Square(8, 8, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(grass)
	for i := uint32(0); i < 8; i++ {
		c, _ := grid.CellIndex(i, 3)
		brd.Set(c, water)
		c, _ = grid.CellIndex(5, i)
		brd.SetWay(c, cell.Way{Kind: road, Width: 8, Links: 0xff})
	}
	atlas := render.NewAtlas()
	for _, k := range []cell.Kind{grass, water} {
		atlas.RegisterAt(k.SpriteID, 4, render.Solid(k.Color))
	}
	atlas.Close()
	space := world.SpaceCfg{Width: 256, Height: 256}
	every := newRenderer(brd, atlas, &RenderState{}, func() Map { return newSimpleMap(brd) })
	once := newRenderer(brd, atlas, &RenderState{}, func() Map { return newSimpleMap(brd) })
	once.space = space
	cam := icamera.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(200, 150)
	cam.ZoomIn(1.5, 0, 0)
	cam.MoveTo(40, 20)
	a, b := make([]byte, 4*200*150), make([]byte, 4*200*150)
	draw := func(r *Renderer, pix []byte, cam camera.Camera) {
		screen := render.NewImage(200, 150)
		render.NewComposer(r).DrawWorld(screen, cam)
		screen.ReadPixels(pix)
	}
	check := func(when string) {
		draw(every, a, cam)
		draw(once, b, cam)
		if !once.stillOn || once.still.Len() == 0 {
			t.Fatalf("%s: the board was not composed once", when)
		}
		for i := 0; i < len(a); i += 4 {
			for k := range 4 {
				if d := int(a[i+k]) - int(b[i+k]); d > 1 || d < -1 {
					t.Fatalf("%s: pixel %d is %v composed once, %v every frame", when, i/4, b[i:i+4], a[i:i+4])
				}
			}
		}
	}
	check("at first")
	c, _ := grid.CellIndex(6, 6)
	brd.Set(c, water)
	check("after a cell changed")
	// a wrapping world, over the seam: the world's first columns drawn on past its last
	once.space.Edges = aabbworld.Torus
	cam = icamera.NewFromSpace(space.Width, space.Height, aabbworld.Torus)
	cam.SetViewport(200, 150)
	cam.ZoomIn(1.5, 0, 0)
	cam.MoveTo(190, 20)
	draw(once, b, cam)
	at := func(x, y int) color.RGBA { i := 4 * (y*200 + x); return color.RGBA{b[i], b[i+1], b[i+2], b[i+3]} }
	if c := at(150, 130); c != water.Color {
		t.Errorf("past the seam, over the second column's water, the screen is %v, want %v", c, water.Color)
	}
	if c := at(150, 30); c != grass.Color {
		t.Errorf("past the seam, over the second column's grass, the screen is %v, want %v", c, grass.Color)
	}
}

// Over a board composed once the GPU draws the grid as the tiles composed every frame show it: a
// square grid's outlined tiles alike, a hex grid's edges within their smoothing.
func TestRenderer_DrawsTheGridOverABoardComposedOnce(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land, Color: color.RGBA{R: 60, G: 160, B: 60, A: 255}}
	atlas := render.NewAtlas()
	atlas.RegisterAt(1, 4, render.Solid(grass.Color))
	atlas.Close()
	for _, c := range []struct {
		name       string
		grid       Grid
		most, mean int // the largest difference of a channel allowed, and of all of them on average, ×100
	}{{"square", DefaultGrids{}.Square(8, 8, 32), 1, 1}, {"hex", DefaultGrids{}.Hex(8, 8, 20), 64, 150}} {
		brd := NewBoard(c.grid, NewTerrainMap())
		brd.SetAll(grass)
		space := world.SpaceCfg{Width: 400, Height: 300}
		every := newRenderer(brd, atlas, &RenderState{ShowGridLines: true}, func() Map { return newSimpleMap(brd) })
		once := newRenderer(brd, atlas, &RenderState{ShowGridLines: true}, func() Map { return newSimpleMap(brd) })
		once.space = space
		cam := icamera.NewFromSpace(space.Width, space.Height, 0)
		cam.SetViewport(200, 150)
		cam.ZoomIn(1.3, 0, 0)
		cam.MoveTo(10, 10)
		a, b := make([]byte, 4*200*150), make([]byte, 4*200*150)
		for _, x := range []struct {
			r   *Renderer
			pix []byte
		}{{every, a}, {once, b}} {
			screen := render.NewImage(200, 150)
			screen.Fill(color.RGBA{A: 255})
			render.NewComposer(x.r).DrawWorld(screen, cam)
			screen.ReadPixels(x.pix)
		}
		if !once.stillOn {
			t.Fatalf("%s: the board was not composed once", c.name)
		}
		most, sum := 0, 0
		for i := range a {
			d := max(int(a[i])-int(b[i]), int(b[i])-int(a[i]))
			most, sum = max(most, d), sum+d
		}
		if most > c.most || 100*sum/len(a) > c.mean {
			t.Errorf("%s: the grid drawn on the GPU differs by %d at most, %.2f on average; want at most %d and %.2f", c.name, most, float64(sum)/float64(len(a)), c.most, float64(c.mean)/100)
		}
	}
}
