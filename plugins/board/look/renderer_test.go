package look_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// the grid on in a line width pixels wide
func gridOf(line color.RGBA, width float32) look.RenderState {
	return look.RenderState{ShowGridLines: true, GridLine: line, GridWidth: width}
}

var black = color.RGBA{A: 255}

// shot draws the sources through cam into a screen w by h, black first, and hands back its pixels
// by screen point
func shot(cam camera.Camera, w, h int, sources ...render.Source) func(x, y float32) color.RGBA {
	screen := render.NewImage(w, h)
	screen.Fill(black)
	render.NewComposer(sources...).DrawWorld(screen, cam)
	pix := make([]byte, 4*w*h)
	screen.ReadPixels(pix)
	return func(x, y float32) color.RGBA {
		if x < 0 || y < 0 || int(x) >= w || int(y) >= h {
			return color.RGBA{} // off the screen
		}
		i := 4 * (int(y)*w + int(x))
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
}

// near reports whether a and b differ by at most d in every channel
func near(a, b color.RGBA, d int) bool {
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}, {a.A, b.A}} {
		if k := int(p[0]) - int(p[1]); k > d || k < -d {
			return false
		}
	}
	return true
}

func TestRenderer_LaysAFlatBoardOnTheGPUAndEachCellAsItChanges(t *testing.T) {
	gputest.Need(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	water := cell.Kind{SpriteID: 2, Cost: 1, Allows: cell.Water}
	road := cell.Kind{SpriteID: 3, Cost: 1, Allows: cell.Land}
	green := color.RGBA{R: 60, G: 160, B: 60, A: 255}
	blue := color.RGBA{R: 40, G: 80, B: 200, A: 255}
	brown := color.RGBA{R: 150, G: 120, B: 80, A: 255}
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	for i := uint32(0); i < 8; i++ {
		brdW.Set(g.At(i, 3), water)
		brdW.SetWay(g.At(5, i), cell.Way{Kind: road, Width: 8, Links: 0xff})
	}
	atlas := render.NewAtlas()
	atlas.Add(grass.SpriteID, 4, render.Solid(green))
	atlas.Add(water.SpriteID, 4, render.Solid(blue))
	atlas.Add(road.SpriteID, 4, render.Solid(brown))
	atlas.Close()
	space := world.SpaceCfg{Width: 256, Height: 256}
	r := look.NewRenderer(brd, atlas, brd.Map(), space)
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(200, 150)
	cam.ZoomIn(1.5, 0, 0)
	cam.MoveTo(100, 20)
	middle := func(at func(x, y float32) color.RGBA, x, y uint32) color.RGBA {
		return at(cam.Project(float32(x)*32+16, float32(y)*32+16, 0))
	}
	at := shot(cam, 200, 150, r)
	if !look.Tiled(r) {
		t.Fatal("the board's tiles were not laid on the GPU")
	}
	for _, c := range []struct {
		x, y uint32
		want color.RGBA
		what string
	}{{4, 1, green, "grass"}, {4, 3, blue, "water"}, {5, 1, brown, "the road across grass"},
		{5, 3, brown, "the road across water"}, {6, 2, green, "grass beside the road"}} {
		if got := middle(at, c.x, c.y); !near(got, c.want, 1) {
			t.Errorf("the middle of cell %d, %d is %v, want %s's %v", c.x, c.y, got, c.what, c.want)
		}
	}
	brdW.Set(g.At(6, 2), water)
	at = shot(cam, 200, 150, r)
	if got := middle(at, 6, 2); !near(got, blue, 1) {
		t.Errorf("a cell turned to water shows %v, want %v", got, blue)
	}
	if got := middle(at, 4, 1); !near(got, green, 1) {
		t.Errorf("a cell left as it was shows %v, want %v", got, green)
	}
	// over a wrapping world's seam
	space.Edges = aabbworld.Torus
	r = look.NewRenderer(brd, atlas, brd.Map(), space)
	cam = cameratest.NewFromSpace(space.Width, space.Height, aabbworld.Torus)
	cam.SetViewport(200, 150)
	cam.ZoomIn(1.5, 0, 0)
	cam.MoveTo(190, 20)
	at = shot(cam, 200, 150, r)
	if c := at(150, 130); c != blue {
		t.Errorf("past the seam, over the second column's water, the screen is %v, want %v", c, blue)
	}
	if c := at(150, 30); c != green {
		t.Errorf("past the seam, over the second column's grass, the screen is %v, want %v", c, green)
	}
}

func TestRenderer_DrawsTheGridOverTheTilesLaidOnTheGPU(t *testing.T) {
	gputest.Need(t)
	green := color.RGBA{R: 60, G: 160, B: 60, A: 255}
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 4, render.Solid(green))
	atlas.Close()
	red := color.RGBA{R: 220, G: 30, B: 30, A: 200}
	for _, c := range []struct {
		name  string
		grid  grid.Grid
		state look.RenderState
	}{
		{"square", grid.DefaultGrids{}.Square(8, 8, 32), gridOf(red, 1)},
		{"hex", grid.DefaultGrids{}.Hex(8, 8, 20), gridOf(red, 1)},
		{"square, 3 pixels", grid.DefaultGrids{}.Square(8, 8, 32), gridOf(red, 3)},
		{"hex, 3 pixels", grid.DefaultGrids{}.Hex(8, 8, 20), gridOf(red, 3)},
	} {
		brd, brdW := board.NewBoard(c.grid)
		brdW.SetAll(grass)
		space := world.SpaceCfg{Width: 400, Height: 300}
		r := look.NewRenderer(brd, atlas, brd.Map(), space)
		*r.State() = c.state
		cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
		cam.SetViewport(200, 150)
		cam.ZoomIn(1.3, 0, 0)
		cam.MoveTo(10, 10)
		at := shot(cam, 200, 150, r)
		cell := c.grid.At(2, 2)
		middle := c.grid.CellCenter(cell)
		outline := c.grid.CellOutline(cell, nil)
		side := [2]float32{float32(outline[0].X+outline[1].X) / 2,
			float32(outline[0].Y+outline[1].Y) / 2}
		if got := at(cam.Project(float32(middle.X), float32(middle.Y), 0)); !near(got, green, 1) {
			t.Errorf("%s: the middle of a cell is %v, want its grass %v", c.name, got, green)
		}
		if got := at(cam.Project(side[0], side[1], 0)); got.R < 150 || got.G > 80 {
			t.Errorf("%s: the middle of a cell's side is %v, want the grid's red over it", c.name,
				got)
		}
	}
}

func TestRenderer_LaysTheTilesInTheOpaquePassAndTheGridOverThem(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd, brdW := board.NewBoard(grid)
	brdW.SetAll(cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land})
	space := world.SpaceCfg{Width: 128, Height: 128}
	r := look.NewRenderer(brd, &flatAtlas{}, brd.Map(), space)
	*r.State() = gridOf(black, 1)
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
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
	passes []render.Pass
}

func (d *dressedGround) DressGround(t render.Target, _ camera.Camera, _ render.Uniforms) {
	d.passes = append(d.passes, t.Pass)
}

type dressedMap struct {
	look.Map
	d *dressedGround
}

func (m dressedMap) Dressing() look.Dressing { return m.d }

func TestRenderer_DressesTheGroundOnceAFrameRightAfterIt(t *testing.T) {
	gputest.Need(t)
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd, brdW := board.NewBoard(grid)
	brdW.SetAll(cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land})
	space := world.SpaceCfg{Width: 128, Height: 128, Edges: aabbworld.Torus}
	d := &dressedGround{Dressing: brd.Map().Dressing()}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 8, render.Solid(color.RGBA{G: 200, A: 255}))
	atlas.Close()
	r := look.NewRenderer(brd, atlas, dressedMap{brd.Map(), d}, space)
	cam := cameratest.NewFromSpace(space.Width, space.Height, aabbworld.Torus)
	cam.MoveTo(64, 64) // across both seams: the board drawn four times
	render.NewComposer(r).DrawWorld(render.NewImage(128, 128), cam)
	if len(d.passes) != 1 || d.passes[0] != render.Opaque {
		t.Errorf("the ground was dressed in %v, want once, in the opaque pass", d.passes)
	}
}

func TestRenderer_DrawsNothingOfABoardOfNoWorld(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd, brdW := board.NewBoard(grid)
	brdW.SetAll(cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land})
	r := look.NewRenderer(brd, &flatAtlas{}, brd.Map(), world.SpaceCfg{})
	cam := cameratest.NewFromSpace(128, 128, 0)
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	places := 0
	f.Places(func(render.Pass, render.At) { places++ })
	if places != 0 || look.Tiled(r) {
		t.Errorf("a board of no world asks for %d places, want none", places)
	}
}
