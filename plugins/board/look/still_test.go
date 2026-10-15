package look_test

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

func TestRenderer_DrawsAFlatBoardComposedOnceAsEveryFrame(t *testing.T) {
	needGPU(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	water := cell.Kind{SpriteID: 2, Cost: 1, Allows: cell.Water}
	road := cell.Kind{SpriteID: 3, Cost: 1, Allows: cell.Land}
	green := color.RGBA{R: 60, G: 160, B: 60, A: 255}
	blue := color.RGBA{R: 40, G: 80, B: 200, A: 255}
	grid := grid.DefaultGrids{}.Square(8, 8, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(grass)
	for i := uint32(0); i < 8; i++ {
		c := grid.At(i, 3)
		brd.Set(c, water)
		c = grid.At(5, i)
		brd.SetWay(c, cell.Way{Kind: road, Width: 8, Links: 0xff})
	}
	atlas := render.NewAtlas()
	atlas.Add(grass.SpriteID, 4, render.Solid(green))
	atlas.Add(water.SpriteID, 4, render.Solid(blue))
	atlas.Add(road.SpriteID, 4, render.Solid(color.RGBA{R: 150, G: 120, B: 80, A: 255}))
	atlas.Close()
	space := world.SpaceCfg{Width: 256, Height: 256}
	every := newRenderer(brd, atlas, look.RenderState{}, brd.Map())
	once := look.NewRenderer(brd, atlas, brd.Map(), space)
	cam := icamera.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(200, 150)
	cam.ZoomIn(1.5, 0, 0)
	cam.MoveTo(40, 20)
	a, b := make([]byte, 4*200*150), make([]byte, 4*200*150)
	draw := func(r *look.Renderer, pix []byte, cam camera.Camera) {
		screen := render.NewImage(200, 150)
		render.NewComposer(r).DrawWorld(screen, cam)
		screen.ReadPixels(pix)
	}
	check := func(when string) {
		draw(every, a, cam)
		draw(once, b, cam)
		if !look.ComposedOnce(once) {
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
	c := grid.At(6, 6)
	brd.Set(c, water)
	check("after a cell changed")
	// over a wrapping world's seam
	space.Edges = aabbworld.Torus
	once = look.NewRenderer(brd, atlas, brd.Map(), space)
	cam = icamera.NewFromSpace(space.Width, space.Height, aabbworld.Torus)
	cam.SetViewport(200, 150)
	cam.ZoomIn(1.5, 0, 0)
	cam.MoveTo(190, 20)
	draw(once, b, cam)
	at := func(x, y int) color.RGBA { i := 4 * (y*200 + x); return color.RGBA{b[i], b[i+1], b[i+2], b[i+3]} }
	if c := at(150, 130); c != blue {
		t.Errorf("past the seam, over the second column's water, the screen is %v, want %v", c, blue)
	}
	if c := at(150, 30); c != green {
		t.Errorf("past the seam, over the second column's grass, the screen is %v, want %v", c, green)
	}
}

func TestRenderer_DrawsTheGridOverABoardComposedOnce(t *testing.T) {
	needGPU(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 4, render.Solid(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Close()
	for _, c := range []struct {
		name       string
		grid       grid.Grid
		most, mean int // the largest difference of a channel allowed, and of all of them on average, ×100
	}{{"square", grid.DefaultGrids{}.Square(8, 8, 32), 1, 1}, {"hex", grid.DefaultGrids{}.Hex(8, 8, 20), 64, 150}} {
		brd := board.NewBoard(c.grid)
		brd.SetAll(grass)
		space := world.SpaceCfg{Width: 400, Height: 300}
		every := newRenderer(brd, atlas, look.RenderState{ShowGridLines: true}, brd.Map())
		once := look.NewRenderer(brd, atlas, brd.Map(), space)
		once.State().ShowGridLines = true
		cam := icamera.NewFromSpace(space.Width, space.Height, 0)
		cam.SetViewport(200, 150)
		cam.ZoomIn(1.3, 0, 0)
		cam.MoveTo(10, 10)
		a, b := make([]byte, 4*200*150), make([]byte, 4*200*150)
		for _, x := range []struct {
			r   *look.Renderer
			pix []byte
		}{{every, a}, {once, b}} {
			screen := render.NewImage(200, 150)
			screen.Fill(color.RGBA{A: 255})
			render.NewComposer(x.r).DrawWorld(screen, cam)
			screen.ReadPixels(x.pix)
		}
		if !look.ComposedOnce(once) {
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

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

func TestRenderer_DrawsTheBoardComposedOnceInTheOpaquePassAndItsGridOverIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land})
	space := world.SpaceCfg{Width: 128, Height: 128}
	r := look.NewRenderer(brd, &flatAtlas{}, brd.Map(), space)
	r.State().ShowGridLines = true
	cam := icamera.NewFromSpace(space.Width, space.Height, 0)
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	var opaque int
	var through []render.At
	f.Places(func(p render.Pass, at render.At) {
		if p == render.Opaque {
			opaque++
		} else {
			through = append(through, at)
		}
	})
	if opaque != 1 || len(through) != 1 || through[0].Z() != look.GridLift {
		t.Errorf("%d opaque places and the grid at %v, want the ground once and its grid at %v",
			opaque, through, look.GridLift)
	}
}

type dressedGround struct {
	look.Dressing
	even   bool
	passes []render.Pass
}

func (d *dressedGround) DressGround(t render.Target, _ camera.Camera, _ render.Uniforms) {
	d.passes = append(d.passes, t.Pass)
}

func (d *dressedGround) EvenLight() (render.Light, bool) { return render.Light{1, 1, 1}, d.even }

type dressedMap struct {
	look.Map
	d *dressedGround
}

func (m dressedMap) Dressing() look.Dressing { return m.d }

func TestRenderer_DressesTheGroundOnceAFrameRightAfterIt(t *testing.T) {
	needGPU(t)
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land})
	space := world.SpaceCfg{Width: 128, Height: 128, Edges: aabbworld.Torus}
	d := &dressedGround{Dressing: brd.Map().Dressing(), even: true}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 8, render.Solid(color.RGBA{G: 200, A: 255}))
	atlas.Close()
	r := look.NewRenderer(brd, atlas, dressedMap{brd.Map(), d}, space)
	cam := icamera.NewFromSpace(space.Width, space.Height, aabbworld.Torus)
	cam.MoveTo(64, 64) // across both seams: the board drawn four times
	render.NewComposer(r).DrawWorld(render.NewImage(128, 128), cam)
	if len(d.passes) != 1 || d.passes[0] != render.Opaque {
		t.Errorf("the ground was dressed in %v, want once, in the opaque pass", d.passes)
	}
	d.even, d.passes = false, nil
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	var lying []render.At
	f.Places(func(p render.Pass, at render.At) {
		if p == render.SeeThrough {
			lying = append(lying, at)
		}
	})
	if len(lying) != 1 || lying[0].Z() != look.GroundLift || look.GroundLift >= look.GridLift {
		t.Errorf("the tiles composed into the frame, the ground is dressed at %v, want once at "+
			"%v, under what lies on the board", lying, look.GroundLift)
	}
}
