package look_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

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

// drawnApart draws r laid on the GPU and its twin composed on the CPU through cam, with the
// other sources, and reports the largest difference of a channel and how many pixels differ by
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
	cpu := look.OnCPU(look.NewRenderer(brd, atlas, mp, space))
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	if most, off := drawnApart(t, laid, cpu, cam, 256, 256); off > 256*256/100 {
		t.Errorf("%d pixels differ from the tiles composed on the CPU by over 24, %d at most; "+
			"want at most 1%% of them, where the edges of the bands fall", off, most)
	}
}

func TestTiles_LayAHexBoardAsTheCPUComposesIt(t *testing.T) {
	needGPU(t)
	g := grid.DefaultGrids{}.Hex(8, 8, 16)
	brd, brdW := board.NewBoard(g)
	kinds := []cell.Kind{{SpriteID: 1, Cost: 1, Allows: cell.Land},
		{SpriteID: 2, Cost: 1, Allows: cell.Land}, {SpriteID: 3, Cost: 1, Allows: cell.Land}}
	for r := range uint32(8) {
		for q := range uint32(8) {
			brdW.Set(g.At(q, r), kinds[(q+2*r)%3])
		}
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 28, render.Hexagon(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Add(render.SpriteID(2), 28, render.Hexagon(color.RGBA{R: 40, G: 80, B: 200, A: 255}))
	atlas.Add(render.SpriteID(3), 28, render.Solid(color.RGBA{R: 200, G: 180, B: 90, A: 255}))
	atlas.Close()
	space := world.SpaceCfg{Width: 400, Height: 300}
	laid := look.NewRenderer(brd, atlas, brd.Map(), space)
	cpu := look.OnCPU(look.NewRenderer(brd, atlas, brd.Map(), space))
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
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(color.RGBA{R: 60, G: 160, B: 60, A: 255}))
	atlas.Add(render.SpriteID(2), 32, render.Solid(color.RGBA{R: 200, G: 190, B: 80, A: 255}))
	atlas.Close()
	wind := [2]float32{30, 12}
	mp := dressed{windy{look.FlatLook(), air.Weather{Wind: wind}}, brd.Map().Dressing()}
	space := world.SpaceCfg{Width: 256, Height: 256}
	laid := look.NewRenderer(brd, atlas, mp, space)
	// composed every frame, as it leans now: composed once, it leant as it stood then
	cpu := look.OnCPU(look.NewRenderer(brd, atlas, mp, world.SpaceCfg{}))
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
