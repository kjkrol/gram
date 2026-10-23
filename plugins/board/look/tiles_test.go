package look_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
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

// dressed is a flat map of l dressed by d
type dressed struct {
	l look.Look
	d look.Dressing
}

func (m dressed) Look() look.Look                               { return m.l }
func (dressed) Heights() ground.Heights                         { return nil }
func (m dressed) Dressing() look.Dressing                       { return m.d }
func (dressed) Top(cell.ID) (corners [4]float32, level float32) { return corners, 0 }

// windy leans what sways with a wind of its own, as the board's atmosphere does
type windy struct {
	look.Look
	w air.Weather
}

func (windy) Sways() bool { return true }

func (l windy) Cell(f *render.Frame, cam camera.Camera, t *look.Tile) {
	if amount, rise := t.Sway(); amount > 0 {
		leaning := *t
		lx, ly := l.w.Sway(f.Time(), (t.X0+t.X1)/2, (t.Y0+t.Y1)/2, amount)
		leaning.X0, leaning.X1 = t.X0+lx*rise, t.X1+lx*rise
		leaning.Y0, leaning.Y1 = t.Y0+ly*rise, t.Y1+ly*rise
		l.Look.Cell(f, cam, &leaning)
		return
	}
	l.Look.Cell(f, cam, t)
}

// weather is a source holding the frame's time and wind still
type weather struct{ wind [2]float32 }

func (weather) Init(*goke.SysInit)                         {}
func (weather) Clock() (time.Duration, bool)               { return 1300 * time.Millisecond, true }
func (w weather) Compose(f *render.Frame, _ camera.Camera) { f.Uniform("Wind", w.wind[0], w.wind[1]) }

// drawnApart draws r laid on the GPU and its twin composed on the CPU through cam (a renderer of
// no world composes its tiles into the frame), with the other sources, and reports the largest difference of a channel and how many pixels differ by
// over 24 in any channel
func drawnApart(t *testing.T, laid, cpu *look.Renderer, cam camera.Camera, w, h int,
	others ...render.Source) (most, off int) {
	t.Helper()
	a, b := make([]byte, 4*w*h), make([]byte, 4*w*h)
	for _, x := range []struct {
		r   *look.Renderer
		pix []byte
	}{{laid, a}, {cpu, b}} {
		screen := render.NewImage(w, h)
		screen.Fill(color.RGBA{A: 255})
		render.NewComposer(append([]render.Source{x.r}, others...)...).DrawWorld(screen, cam)
		screen.ReadPixels(x.pix)
	}
	if !look.Tiled(laid) {
		t.Fatal("the board's tiles were not laid on the GPU")
	}
	for i := 0; i < len(a); i += 4 {
		worst := 0
		for k := range 4 {
			worst = max(worst, int(a[i+k])-int(b[i+k]), int(b[i+k])-int(a[i+k]))
		}
		most = max(most, worst)
		if worst > 24 {
			off++
		}
	}
	t.Logf("%d pixels differ by over 24, %d at most", off, most)
	if dir := os.Getenv("GRAM_TILES_PNG"); dir != "" {
		for _, x := range []struct {
			name string
			pix  []byte
		}{{"gpu", a}, {"cpu", b}} {
			img := &image.RGBA{Pix: x.pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
			f, _ := os.Create(dir + "/" + t.Name() + "-" + x.name + ".png")
			_ = png.Encode(f, img)
			f.Close()
		}
	}
	return most, off
}

func TestTiles_LayTheCoversAndTheWaysAsTheCPUComposesThem(t *testing.T) {
	needGPU(t)
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
	atlas.Add(render.SpriteID(1), 32, render.Solid(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Add(render.SpriteID(2), 32, render.Solid(color.RGBA{R: 240, G: 240, B: 250, A: 255}))
	atlas.Add(render.SpriteID(3), 32, render.Solid(color.RGBA{R: 150, G: 110, B: 70, A: 255}))
	atlas.Close()
	d := draw.NewBands(m)
	d.Cover(mark, 2)
	space := world.SpaceCfg{Width: 256, Height: 256}
	mp := dressed{look.FlatLook(), d}
	laid := look.NewRenderer(brd, atlas, mp, space)
	cpu := look.NewRenderer(brd, atlas, mp, world.SpaceCfg{})
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	if most, off := drawnApart(t, laid, cpu, cam, 256, 256); off > 256*256/100 {
		t.Errorf("%d pixels differ from the tiles composed on the CPU by over 24, %d at most; "+
			"want at most 1%% of them, where the edges of the bands fall", off, most)
	}
}

// a hex's box reaches over its neighbours': each cell's tile and the covers on it lie over the
// cells before it and under those after it, the ways over them all
func TestTiles_LayAHexBoardAsTheCPUComposesIt(t *testing.T) {
	needGPU(t)
	const mark tag.Tag[effect.States] = 3
	g := grid.DefaultGrids{}.Hex(8, 8, 16)
	brd, brdW := board.NewBoard(g)
	kinds := []cell.Kind{{SpriteID: 1, Cost: 1, Allows: cell.Land},
		{SpriteID: 2, Cost: 1, Allows: cell.Land}, {SpriteID: 3, Cost: 1, Allows: cell.Land}}
	road := cell.Kind{SpriteID: 5, Cost: 1, Allows: cell.Land}
	m := marked{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{}}
	for r := range uint32(8) {
		for q := range uint32(8) {
			brdW.Set(g.At(q, r), kinds[(q+2*r)%3])
			if (q+r)%4 == 1 {
				m.cells[g.At(q, r)] = true
			}
		}
		brdW.SetWay(g.At(3, r), cell.Way{Kind: road, Width: 7, Links: 0x3f})
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 28, render.Hexagon(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Add(render.SpriteID(2), 28, render.Hexagon(color.RGBA{R: 40, G: 80, B: 200, A: 255}))
	atlas.Add(render.SpriteID(3), 28, render.Solid(color.RGBA{R: 200, G: 180, B: 90, A: 255}))
	atlas.Add(render.SpriteID(4), 28, render.Solid(color.RGBA{R: 240, G: 240, B: 250, A: 200}))
	atlas.Add(render.SpriteID(5), 28, render.Solid(color.RGBA{R: 150, G: 110, B: 70, A: 255}))
	atlas.Close()
	d := draw.NewBands(m)
	d.Cover(mark, 4)
	mp := dressed{look.FlatLook(), d}
	space := world.SpaceCfg{Width: 400, Height: 300}
	laid := look.NewRenderer(brd, atlas, mp, space)
	cpu := look.NewRenderer(brd, atlas, mp, world.SpaceCfg{})
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(320, 240)
	if most, off := drawnApart(t, laid, cpu, cam, 320, 240); off > 320*240/100 {
		t.Errorf("%d pixels differ from the hexes composed on the CPU by over 24, %d at most; want "+
			"at most 1%% of them, along the hexes' rims", off, most)
	}
}

func TestTiles_LeanWhatSwaysAsTheCPUComposesIt(t *testing.T) {
	needGPU(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	reeds := cell.Kind{SpriteID: 2, Cost: 1, Allows: cell.Land, Sway: 1, Height: 20}
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	for _, at := range [][2]uint32{{2, 2}, {3, 2}, {5, 4}, {1, 6}, {6, 6}} {
		brdW.Set(g.At(at[0], at[1]), reeds)
	}
	// covers on what sways and beside it: each lies over its own tile and under the next one's
	const mark tag.Tag[effect.States] = 3
	m := marked{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{}}
	for _, at := range [][2]uint32{{2, 2}, {4, 2}, {2, 3}, {5, 5}, {6, 6}, {0, 0}} {
		m.cells[g.At(at[0], at[1])] = true
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Add(render.SpriteID(2), 32, render.Solid(color.RGBA{R: 200, G: 190, B: 80, A: 255}))
	atlas.Add(render.SpriteID(4), 32, render.Solid(color.RGBA{R: 240, G: 240, B: 250, A: 200}))
	atlas.Close()
	d := draw.NewBands(m)
	d.Cover(mark, 4)
	wind := [2]float32{30, 12}
	mp := dressed{windy{look.FlatLook(), air.Weather{Wind: wind}}, d}
	space := world.SpaceCfg{Width: 256, Height: 256}
	laid := look.NewRenderer(brd, atlas, mp, space)
	cpu := look.NewRenderer(brd, atlas, mp, world.SpaceCfg{})
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	if most, off := drawnApart(t, laid, cpu, cam, 256, 256, weather{wind}); off > 256*256/100 {
		t.Errorf("%d pixels differ from the leaning tiles composed on the CPU by over 24, %d at "+
			"most; want at most 1%% of them", off, most)
	}
}

// the tiles composed on the CPU lie only as a camera from above lays rectangles: under a camera
// turned and tilted, every pixel is the cell under it
func TestTiles_LayTheBoardUnderATurnedAndTiltedCamera(t *testing.T) {
	needGPU(t)
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
	r := look.NewRenderer(brd, atlas, dressed{look.FlatLook(), bands},
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
	needGPU(t)
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

// the whole board from above, as the minimap shows it, laid as the CPU composed it once
func TestTiles_LayTheWholeBoardFromAboveAsTheCPUComposesIt(t *testing.T) {
	needGPU(t)
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
	atlas.Add(render.SpriteID(1), 32, render.Solid(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Add(render.SpriteID(2), 32, render.Solid(color.RGBA{R: 240, G: 240, B: 250, A: 255}))
	atlas.Add(render.SpriteID(3), 32, render.Solid(color.RGBA{R: 150, G: 110, B: 70, A: 255}))
	atlas.Close()
	d := draw.NewBands(m)
	d.Cover(mark, 2)
	mp := dressed{look.FlatLook(), d}
	light := render.Light{0.8, 0.9, 1}
	for _, px := range []float32{1, 0.5} {
		n := int(256 * px)
		u := render.UniformsOf(map[string]any{"Pixel": []float32{1 / px}})
		a, b := make([]byte, 4*n*n), make([]byte, 4*n*n)
		gpuImg := render.NewImage(n, n)
		look.NewRenderer(brd, atlas, mp, world.SpaceCfg{Width: 256, Height: 256}).
			Whole(gpuImg, px, light, u)
		gpuImg.ReadPixels(a)
		cpuImg := render.NewImage(n, n)
		still := render.NewStill()
		still.Compose(256, 256, look.NewRenderer(brd, atlas, mp, world.SpaceCfg{}).Compose)
		still.Draw(render.Target{Screen: cpuImg}, nil, u, px, 0, 0, light)
		cpuImg.ReadPixels(b)
		off := 0
		for i := 0; i < len(a); i += 4 {
			for k := range 4 {
				if d := int(a[i+k]) - int(b[i+k]); d > 24 || d < -24 {
					off++
					break
				}
			}
		}
		t.Logf("at %v a unit %d pixels differ by over 24", px, off)
		if off > n*n/100 {
			t.Errorf("at %v a unit %d pixels differ from the board composed on the CPU, want at "+
				"most 1%% of them, where the edges of the bands fall", px, off)
		}
	}
}
