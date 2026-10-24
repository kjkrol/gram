package look_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// marked is a board some of whose cells are under the effect mark
type marked struct {
	*board.BoardRead
	mark  tag.Tag[effect.States]
	cells map[cell.ID]bool
}

func (m marked) States(c cell.ID) tag.Tags[effect.States] {
	if m.cells[c] {
		return tag.Tags[effect.States](0).With(m.mark)
	}
	return 0
}

// dressed is a flat map laid as l says, dressed by d
type dressed struct {
	l look.Look
	d look.Dressing
}

func (m dressed) Look() look.Look                               { return m.l }
func (dressed) Heights() ground.Heights                         { return nil }
func (m dressed) Dressing() look.Dressing                       { return m.d }
func (dressed) Top(cell.ID) (corners [4]float32, level float32) { return corners, 0 }

// weather is a source holding the frame's time and wind still
type weather struct{ wind [2]float32 }

func (weather) Clock() (time.Duration, bool)               { return 1300 * time.Millisecond, true }
func (w weather) Compose(f *render.Frame, _ camera.Camera) { f.Uniform("Wind", w.wind[0], w.wind[1]) }

var (
	green = color.RGBA{R: 60, G: 160, B: 60, A: 255}
	snow  = color.RGBA{R: 240, G: 240, B: 250, A: 200}
	brown = color.RGBA{R: 150, G: 110, B: 70, A: 255}
)

// over is c with the cover snow laid over it
func over(c color.RGBA) color.RGBA {
	a := float32(snow.A) / 255
	mix := func(s, d uint8) uint8 { return uint8(float32(s)*a + float32(d)*(1-a) + 0.5) }
	return color.RGBA{mix(snow.R, c.R), mix(snow.G, c.G), mix(snow.B, c.B), 255}
}

func TestTiles_LayTheCoversAndTheWaysOverTheTiles(t *testing.T) {
	gputest.Need(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	road := cell.Kind{SpriteID: 3, Cost: 1, Allows: cell.Land}
	const mark tag.Tag[effect.States] = 3
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	m := marked{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{}}
	for _, at := range [][2]uint32{{2, 2}, {3, 2}, {2, 3}, {3, 3}, {4, 3}, {4, 4}, {5, 5}, {0, 7}} {
		m.cells[g.At(at[0], at[1])] = true
	}
	for i := range uint32(8) {
		brdW.SetWay(g.At(i, i), cell.Way{Kind: road, Width: 9, Links: 0xff})
		brdW.SetWay(g.At(6, i), cell.Way{Kind: road, Width: 6, Links: 0x03, Fade: 0.4})
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(green))
	atlas.Add(render.SpriteID(2), 32, render.Solid(snow))
	atlas.Add(render.SpriteID(3), 32, render.Solid(brown))
	atlas.Close()
	d := draw.NewBands(m)
	d.Cover(mark, 2)
	space := world.SpaceCfg{Width: 256, Height: 256}
	r := look.NewRenderer(brd, atlas, dressed{look.Flat, d}, space)
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	at := shot(cam, 256, 256, r)
	for _, c := range []struct {
		x, y float32
		want color.RGBA
		what string
	}{
		{3*32 + 16, 2*32 + 16, over(green), "a covered cell's middle"},
		{1*32 + 16, 5*32 + 16, green, "a bare cell's middle, away from the covers"},
		{3*32 + 30, 2*32 + 2, green, "the covered block's outer corner, the line cutting it"},
		{3 * 32, 2*32 + 16, over(green), "between two covered cells, unbroken"},
		{4*32 + 16, 4*32 + 16, brown, "the way across a covered cell, over its cover"},
		{1*32 + 16, 1*32 + 16, brown, "the way across a bare cell"},
	} {
		if got := at(c.x, c.y); !near(got, c.want, 3) {
			t.Errorf("%s is %v, want %v", c.what, got, c.want)
		}
	}
	// the way fading out, as far as it shows
	if got := at(6*32+16, 1*32+16); near(got, brown, 6) || near(got, green, 6) {
		t.Errorf("a way faded by 0.4 is %v, want it between the way's %v and the grass's %v", got,
			brown, green)
	}
}

// a hex's box reaches over its neighbours': each cell's tile and the cover on it lie over the
// cells laid before it, row by row, and under those after it
func TestTiles_LayAHexBoardsTilesInTheOrderOfItsCells(t *testing.T) {
	gputest.Need(t)
	const mark tag.Tag[effect.States] = 3
	g := grid.DefaultGrids{}.Hex(8, 8, 16)
	brd, brdW := board.NewBoard(g)
	colours := []color.RGBA{green, {R: 40, G: 80, B: 200, A: 255}, {R: 200, G: 180, B: 90, A: 255}}
	kinds := make([]cell.Kind, len(colours))
	atlas := render.NewAtlas()
	for i, c := range colours {
		kinds[i] = cell.Kind{SpriteID: render.SpriteID(i + 1), Cost: 1, Allows: cell.Land}
		atlas.Add(kinds[i].SpriteID, 28, render.Solid(c))
	}
	atlas.Add(render.SpriteID(4), 28, render.Solid(snow))
	atlas.Close()
	m := marked{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{}}
	for r := range uint32(8) {
		for q := range uint32(8) {
			brdW.Set(g.At(q, r), kinds[(q+2*r)%3])
			if (q+r)%4 == 1 {
				m.cells[g.At(q, r)] = true
			}
		}
	}
	d := draw.NewBands(m)
	d.Cover(mark, 4)
	space := world.SpaceCfg{Width: 400, Height: 300}
	r := look.NewRenderer(brd, atlas, dressed{look.Flat, d}, space)
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(320, 240)
	at := shot(cam, 320, 240, r)
	w, h := brd.CellBounds()
	checked := 0
	for sy := float32(1); sy < 240; sy += 3 {
	pixels:
		for sx := float32(1); sx < 320; sx += 3 {
			x, y := cam.FromScreen(sx+0.5, sy+0.5)
			// the last cell, row by row, whose box holds the point, away from every box's edge
			top := -1
			for i := range brd.CellCount() {
				mid := brd.CellCenter(brd.At(uint32(i%8), uint32(i/8)))
				dx, dy := float64(x)-mid.X, float64(y)-mid.Y
				ex, ey := w/2-max(dx, -dx), h/2-max(dy, -dy)
				if ex > -1 && ey > -1 && (ex < 1 || ey < 1) {
					continue pixels
				}
				if ex > 0 && ey > 0 {
					top = i
				}
			}
			if top < 0 {
				continue
			}
			c := brd.At(uint32(top%8), uint32(top/8))
			want := colours[(top%8+2*(top/8))%3]
			if m.cells[c] {
				want = over(want)
			}
			if got := at(sx, sy); !near(got, want, 3) {
				t.Fatalf("the pixel (%v, %v) is %v, want %v of the cell %d laid last over it",
					sx, sy, got, want, top)
			}
			checked++
		}
	}
	if checked < 1000 {
		t.Errorf("only %d pixels checked, want the board in view", checked)
	}
}

// what sways leans with the wind, its tile and the cover on it: over the cells laid before it,
// under those after it
func TestTiles_LeanWhatSwaysWithTheWind(t *testing.T) {
	gputest.Need(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	reeds := cell.Kind{SpriteID: 2, Cost: 1, Allows: cell.Land, Sway: 1, Height: 20}
	yellow := color.RGBA{R: 200, G: 190, B: 80, A: 255}
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	brdW.Set(g.At(5, 3), reeds)
	brdW.Set(g.At(6, 5), reeds)
	const mark tag.Tag[effect.States] = 3
	m := marked{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{g.At(6, 5): true}}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(green))
	atlas.Add(render.SpriteID(2), 32, render.Solid(yellow))
	atlas.Add(render.SpriteID(4), 32, render.Solid(snow))
	atlas.Close()
	d := draw.NewBands(m)
	d.Cover(mark, 4)
	wind := [2]float32{-50, -20} // to the north-west, over the cells before
	space := world.SpaceCfg{Width: 256, Height: 256}
	r := look.NewRenderer(brd, atlas, dressed{look.Swaying, d}, space)
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	at := shot(cam, 256, 256, weather{wind}, r)
	for _, c := range []struct {
		x, y    uint32
		covered bool
	}{{5, 3, false}, {6, 5, true}} {
		mx, my := float32(c.x)*32+16, float32(c.y)*32+16
		lx, ly := air.Weather{Wind: wind}.Sway(1.3, mx, my, 1)
		lx, ly = lx*20, ly*20
		if lx > -4 {
			t.Fatalf("the reeds at %d, %d lean %v, %v; want a lean to test", c.x, c.y, lx, ly)
		}
		// past the cell's west edge, inside the leant tile: the reeds, and the cover fading in
		// over them where it lies
		got := at(mx-16+lx/2, my)
		if covered := got.B > yellow.B+20; !near(got, yellow, 3) && !covered ||
			covered != c.covered || got.R < yellow.R-3 {
			t.Errorf("past the reeds' west edge at %d, %d the screen is %v, want them leant over "+
				"it, %v, covered %v", c.x, c.y, got, yellow, c.covered)
		}
		// inside the cell's box by its east edge, left behind
		if got := at(mx+16+lx/2, my); got.R >= yellow.R-3 {
			t.Errorf("by the reeds' east edge at %d, %d the screen is %v, want them leant away",
				c.x, c.y, got)
		}
	}
}

// under a camera turned and tilted every pixel is the cell under it
func TestTiles_LayTheBoardUnderATurnedAndTiltedCamera(t *testing.T) {
	gputest.Need(t)
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	kinds := []cell.Kind{{SpriteID: 1, Cost: 1, Allows: cell.Land},
		{SpriteID: 2, Cost: 1, Allows: cell.Water}}
	colours := []color.RGBA{{R: 60, G: 160, B: 60, A: 255}, {R: 40, G: 80, B: 200, A: 255}}
	for y := range uint32(8) {
		for x := range uint32(8) {
			brdW.Set(g.At(x, y), kinds[(x+y)%2])
		}
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(colours[0]))
	atlas.Add(render.SpriteID(2), 32, render.Solid(colours[1]))
	atlas.Close()
	space := world.SpaceCfg{Width: 256, Height: 256}
	r := look.NewRenderer(brd, atlas, brd.Map(), space)
	cam := cameratest.NewFromAbove(space.Width, space.Height)
	cameratest.Turn(cam, 0.785)
	cameratest.Tilt(cam, -0.6)
	const w, h = 320, 240
	cam.SetViewport(w, h)
	cam.CenterOn(128, 128, 0)
	screen := render.NewImage(w, h)
	render.NewComposer(r).DrawWorld(screen, cam)
	pix := make([]byte, 4*w*h)
	screen.ReadPixels(pix)
	if !look.Tiled(r) {
		t.Fatal("the board's tiles were not laid on the GPU")
	}
	checked := 0
	for sy := 4; sy < h; sy += 8 {
		for sx := 4; sx < w; sx += 8 {
			x, y := cam.Unproject(float32(sx)+0.5, float32(sy)+0.5, 0)
			cx, cy := int(x)/32, int(y)/32
			fx, fy := x-float32(cx*32), y-float32(cy*32)
			if x < 0 || y < 0 || cx > 7 || cy > 7 || fx < 3 || fy < 3 || fx > 29 || fy > 29 {
				continue // off the board or near a cell's edge
			}
			want := colours[(cx+cy)%2]
			i := 4 * (sy*w + sx)
			if got := (color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}); got != want {
				t.Fatalf("the pixel (%d, %d) over the cell (%d, %d) is %v, want %v", sx, sy, cx,
					cy, got, want)
			}
			checked++
		}
	}
	if checked < 100 {
		t.Errorf("only %d pixels lay over the board, want the view to see it", checked)
	}
}

func (s sighted) Rays() (camera.RayField, bool) {
	r, ok := s.Camera.(camera.Rays)
	if !ok {
		return camera.RayField{}, false
	}
	return r.Rays()
}

// the tiles of a 4 by 4 board of cells 32 wide, a way across its first row and a cover on its
// last, laid on the GPU through a camera seeing its halves as d says
func laidSeen(t *testing.T, d camera.Detail) func(x, y int) color.RGBA {
	t.Helper()
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	brd, brdW := board.NewBoard(g)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	road := cell.Kind{SpriteID: 3, Cost: 1, Allows: cell.Land}
	brdW.SetAll(grass)
	const mark tag.Tag[effect.States] = 3
	m := marked{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{}}
	for x := range uint32(4) {
		brdW.SetWay(g.At(x, 0), cell.Way{Kind: road, Width: 12, Links: 0x0c})
		m.cells[g.At(x, 3)] = true
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Add(render.SpriteID(2), 32, render.Solid(color.RGBA{R: 240, G: 240, B: 250, A: 255}))
	atlas.Add(render.SpriteID(3), 32, render.Solid(color.RGBA{R: 150, G: 110, B: 70, A: 255}))
	atlas.Close()
	bands := draw.NewBands(m)
	bands.Cover(mark, 2)
	cam := sighted{Camera: cameratest.NewFromSpace(128, 128, 0), d: d}
	v := view.NewSeen(cam, halves{})
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 128, Height: 128, BucketSize: 32})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 128, 128).Update(nil, 0)
	r := look.NewRenderer(brd, atlas, dressed{look.Flat, bands},
		world.SpaceCfg{Width: 128, Height: 128}).
		WithSeen(func(camera.Camera) *view.View { return v })
	screen := render.NewImage(128, 128)
	screen.Fill(color.RGBA{A: 255})
	render.NewComposer(r).DrawWorld(screen, cam)
	pix := make([]byte, 4*128*128)
	screen.ReadPixels(pix)
	if !look.Tiled(r) {
		t.Fatal("the board's tiles were not laid on the GPU")
	}
	return func(x, y int) color.RGBA {
		i := 4 * (y*128 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
}

func TestTiles_LayTheCellsOfTheSectorsSeenAtTheirLevels(t *testing.T) {
	gputest.Need(t)
	grass := color.RGBA{R: 60, G: 160, B: 60, A: 255}
	road := color.RGBA{R: 150, G: 110, B: 70, A: 255}
	snow := color.RGBA{R: 240, G: 240, B: 250, A: 255}
	at := laidSeen(t, camera.Detail{Coarse: 0.5})
	for _, c := range []struct {
		x, y int
		want color.RGBA
		what string
	}{
		{16, 16, road, "the way in the left half, seen exactly"},
		{80, 16, grass, "the way in the right half, seen coarsely"},
		{16, 112, snow, "the cover in the left half"},
		{112, 112, grass, "the cover in the right half"},
	} {
		if got := at(c.x, c.y); got != c.want {
			t.Errorf("%s is %v, want %v", c.what, got, c.want)
		}
	}
	at = laidSeen(t, camera.Detail{Sight: 60, Fog: 40})
	if got := at(96, 16); got != (color.RGBA{A: 255}) {
		t.Errorf("the right half, out of sight, is %v, want nothing drawn", got)
	}
	// the middles 23 and 51 off the left edge's middle, into the fog from 20 to 60
	near, far := at(16, 48), at(48, 80)
	if far.G >= grass.G*3/4 || near.G <= far.G {
		t.Errorf("the cells 23 and 51 off are %v and %v, want them fading into the fog, the "+
			"farther more", near, far)
	}
}

// the whole board from above, as the minimap shows it, is the board a camera from above shows
func TestTiles_LayTheWholeBoardFromAboveAsACameraShowsIt(t *testing.T) {
	gputest.Need(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	road := cell.Kind{SpriteID: 3, Cost: 1, Allows: cell.Land}
	const mark tag.Tag[effect.States] = 3
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	m := marked{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{}}
	for _, at := range [][2]uint32{{2, 2}, {3, 2}, {2, 3}, {5, 5}} {
		m.cells[g.At(at[0], at[1])] = true
	}
	for i := range uint32(8) {
		brdW.SetWay(g.At(i, i), cell.Way{Kind: road, Width: 9, Links: 0xff})
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(green))
	atlas.Add(render.SpriteID(2), 32, render.Solid(snow))
	atlas.Add(render.SpriteID(3), 32, render.Solid(brown))
	atlas.Close()
	d := draw.NewBands(m)
	d.Cover(mark, 2)
	space := world.SpaceCfg{Width: 256, Height: 256}
	mp := dressed{look.Flat, d}
	for _, px := range []float32{1, 0.5} {
		n := int(256 * px)
		u := render.UniformsOf(map[string]any{"Pixel": []float32{1 / px}})
		whole := render.NewImage(n, n)
		whole.Fill(black)
		look.NewRenderer(brd, atlas, mp, space).Whole(whole, px, render.Light{1, 1, 1}, u)
		a := make([]byte, 4*n*n)
		whole.ReadPixels(a)
		cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
		cam.SetViewport(float32(n), float32(n))
		if px < 1 {
			cam.ZoomOut(1/px, 0, 0)
		}
		at := shot(cam, n, n, look.NewRenderer(brd, atlas, mp, space))
		for y := range n {
			for x := range n {
				i := 4 * (y*n + x)
				want := at(float32(x), float32(y))
				if got := (color.RGBA{a[i], a[i+1], a[i+2], a[i+3]}); !near(got, want, 1) {
					t.Fatalf("at %v a unit the pixel (%d, %d) is %v, want %v as the camera shows "+
						"it", px, x, y, got, want)
				}
			}
		}
	}
}
