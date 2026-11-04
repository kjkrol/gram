package terrain

import (
	"image"
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

type bumped struct{}

func (bumped) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	heights = make([]float32, 17*17)
	heights[10*17+4] = 40
	return 17, 17, 32, heights, true
}
func (bumped) Version() uint64 { return 0 }

type grey struct{ img *render.Image }

func (g *grey) Surface() Painted {
	if g.img == nil {
		g.img = render.NewImage(16, 16)
		g.img.Fill(color.RGBA{128, 128, 128, 255})
	}
	return Painted{Albedo: g.img, Px: 1, Reach: 96}
}

type noAir struct{}

func (noAir) Air() air.Weather { return air.Weather{} }

func (noAir) Distant() light.Distant { return light.Distant{} }

// straight down from 200 up, a screen pixel 8 world units
type downward struct{ camera.Camera }

func (downward) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{0, 0, 200}, DX: [3]float32{8, 0, 0}, DY: [3]float32{0, 8, 0}, Dir: [3]float32{0, 0, -1}}, true
}

func TestTerrain_DrawsTheLevelEvenlyAndABumpWhereItStands(t *testing.T) {
	gputest.Need(t)
	r := New(bumped{}, &grey{}, noAir{}, Config{})
	screen := render.NewImage(64, 64)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0.3, 0.2, 1}, "LightStrength": []float32{1}, "LightColor": []float32{1, 1, 1}, "Ambience": []float32{0.2, 0.2, 0.2}})
	depth := render.NewDepth()
	screen.ClearDepth(depth)
	r.Draw(render.Target{Screen: screen, Depth: depth},
		downward{cameratest.NewFromSpace(64, 64, 0)}, u)
	pix := make([]byte, 4*64*64)
	screen.ReadPixels(pix)
	red := func(x, y int) byte { return pix[4*(y*64+x)] }
	level := red(5, 5)
	for _, p := range [][2]int{{60, 5}, {5, 60}, {60, 60}, {40, 20}, {30, 50}} {
		if v := red(p[0], p[1]); v < level-2 || v > level+2 {
			t.Errorf("level ground at pixel %v is %d, at (5, 5) %d: want lit alike", p, v, level)
		}
	}
	if v := red(14, 40); v >= level-3 && v <= level+3 {
		t.Errorf("the raised corner's slope at pixel (14, 40) is %d, like the level %d: no bump there", v, level)
	}
}

func TestTerrainShaders_Compile(t *testing.T) {
	gputest.Need(t)
	if len(Shaders()) != int(lod.L4) {
		t.Fatalf("%d shaders, want one a level drawn, %d", len(Shaders()), lod.L4)
	}
	for level, s := range Shaders() {
		if err := s.Compile(); err != nil {
			t.Errorf("L%d: %v", level, err)
		}
	}
}

// level ground of 16 by 16 cells
type flatland struct{}

func (flatland) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return 17, 17, 32, make([]float32, 17*17), true
}
func (flatland) Version() uint64 { return 0 }

// grey ground, Px pixels a cell, with the sea on the cell (4, 10), painted blue, shining 0.9, a
// mouth glinting over it
type wetCell struct{ albedo, water *render.Image }

const wetPx = 4

var (
	landColour = color.RGBA{128, 128, 128, 255}
	seaColour  = color.RGBA{30, 60, 200, 255}
)

func (w *wetCell) Surface() Painted {
	if w.albedo == nil {
		w.albedo = render.NewImage(16*wetPx, 16*wetPx)
		w.albedo.Fill(landColour)
		w.albedo.SubImage(image.Rect(4*wetPx, 10*wetPx, 5*wetPx, 11*wetPx)).Fill(seaColour)
		// the layers 4 pixels a cell in quadrants of 64: the shine layer's at the right, the
		// mouths' below
		w.water = render.NewImage(128, 128)
		w.water.Fill(color.RGBA{0, 0, 0, 255})
		w.water.SubImage(image.Rect(64+16, 40, 64+20, 44)).Fill(color.RGBA{0, 230, 255, 255})
		w.water.SubImage(image.Rect(16, 64+40, 20, 64+44)).Fill(color.RGBA{230, 255, 0, 255})
	}
	wet := make([]bool, 16*16)
	wet[10*16+4] = true
	return Painted{Albedo: w.albedo, Px: wetPx, Water: w.water, WaterPx: 4, Reach: 96, Wet: wet,
		Wetness: 1}
}

// one sector, the whole world, seen at level
type atLevel struct{ level lod.Level }

func (*atLevel) Len() int  { return 1 }
func (*atLevel) Cols() int { return 1 }
func (*atLevel) Box(int) geom.AABB {
	return geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(512, 512))
}
func (*atLevel) Of(float32, float32) int              { return 0 }
func (*atLevel) Ladder() lod.Ladder                   { return lod.Ladder{} }
func (g *atLevel) Level(camera.Camera, int) lod.Level { return g.level }

// straight down over the wet cell, a screen pixel 2 world units: its cell spans (32..48, 32..48)
type overSea struct{ camera.Camera }

func (overSea) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{64, 256, 200}, DX: [3]float32{2, 0, 0},
		DY: [3]float32{0, 2, 0}, Dir: [3]float32{0, 0, -1}}, true
}

// leveledDraw draws surface over the flat land through cam, the world one sector seen at the
// level it is set to, under u, and reads the screen back
func leveledDraw(t *testing.T, surface Surface, cam camera.Camera, u render.Uniforms) (
	draw func(level lod.Level) (runs []span, pix []byte)) {
	t.Helper()
	return leveledDrawWith(t, surface, cam, u, nil)
}

func TestTerrain_DrawsEachLevelWithoutWhatItsWordDrops(t *testing.T) {
	gputest.Need(t)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0.3, 0.2, 1},
		"LightStrength": []float32{1}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{0.2, 0.2, 0.2}, "Fog": []float32{0.5, 0.6, 0.9},
		"SkyOverhead": []float32{0.2, 0.4, 0.9}, "Toward": []float32{0, 0.98, 0.2},
		"Pixel": []float32{2}})
	draw := leveledDraw(t, &wetCell{}, overSea{cameratest.NewFromSpace(64, 64, 0)}, u)
	at := func(pix []byte, x, y int) color.RGBA {
		i := 4 * (y*64 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	var land, sea [lod.L4]color.RGBA
	for level := lod.L0; level < lod.L4; level++ {
		runs, pix := draw(level)
		// at L3 the world's coarse run and the skirt, which follows it, are one
		if n := len(runs); runs[0].level != level || runs[n-1].level != lod.L3 ||
			n != 2 && level != lod.L3 {
			t.Fatalf("runs %v, want the world's at L%d and the skirt's at L3", runs, level)
		}
		land[level], sea[level] = at(pix, 5, 5), at(pix, 40, 40)
		if land[level].A != 255 || sea[level].A != 255 {
			t.Fatalf("L%d: the land %v and the sea %v, want both drawn", level, land[level],
				sea[level])
		}
	}
	for level := lod.L1; level < lod.L4; level++ {
		if !near(land[level], land[lod.L0], 1) {
			t.Errorf("the land at L%d is %v, at L0 %v: want alike without a baked shade, a "+
				"lamp, a grain or a grid", level, land[level], land[lod.L0])
		}
		if near(sea[level], sea[level-1], 2) {
			t.Errorf("the sea at L%d is %v, as at L%d: want it without what L%d drops", level,
				sea[level], level-1, level)
		}
	}
	// the sheet: the sea's albedo in the light the land is in, no water
	light := float32(land[lod.L3].R) / float32(landColour.R)
	in := func(v byte) byte { return byte(float32(v)*light + 0.5) }
	want := color.RGBA{in(seaColour.R), in(seaColour.G), in(seaColour.B), 255}
	if !near(sea[lod.L3], want, 3) {
		t.Errorf("the sea as a sheet is %v, want the albedo's %v in the light", sea[lod.L3], want)
	}
	runs, pix := draw(lod.L4)
	if len(runs) != 1 || runs[0].level != lod.L3 {
		t.Errorf("runs at L4 %v, want the skirt alone, as a sheet", runs)
	}
	for k := 3; k < len(pix); k += 4 {
		if pix[k] != 0 {
			t.Fatalf("the pixel %d is drawn at L4, want nothing", k/4)
			break
		}
	}
}

// grey ground under the grid in red, two pixels wide, from cells 6 pixels across
type gridded struct{ grey }

func (g *gridded) Surface() Painted {
	p := g.grey.Surface()
	p.Grid, p.GridLine, p.GridWidth = 6, [4]float32{1, 0, 0, 1}, 2
	return p
}

func TestTerrain_DrawsTheGridAtL0Alone(t *testing.T) {
	gputest.Need(t)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0, 0, 1},
		"LightStrength": []float32{0}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{1, 1, 1}})
	draw := leveledDraw(t, &gridded{}, overSea{cameratest.NewFromSpace(64, 64, 0)}, u)
	red := color.RGBA{255, 0, 0, 255}
	at := func(pix []byte, x, y int) color.RGBA {
		i := 4 * (y*64 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	// a pixel 2 world units: the cells' edge at 96 runs between the pixels 15 and 16
	for level := lod.L0; level < lod.L4; level++ {
		_, pix := draw(level)
		edge, inside := at(pix, 16, 5), at(pix, 8, 5)
		if want := level == lod.L0; near(edge, red, 2) != want || near(inside, red, 2) {
			t.Errorf("L%d: the cells' edge %v, a cell's inside %v; want the grid's red on the "+
				"edge %v", level, edge, inside, want)
		}
	}
}

// a sheet of colours a pixel each, Px pixels a cell
type mottled struct{ img *render.Image }

const mottledPx = 4

func mottle(x, y int) color.RGBA {
	return color.RGBA{byte(40 + 13*x%200), byte(30 + 29*y%200), byte(50 + 7*(x+y)%200), 255}
}

func (m *mottled) Surface() Painted {
	if m.img == nil {
		n := 16 * mottledPx
		m.img = render.NewImage(n, n)
		pix := make([]byte, 0, 4*n*n)
		for y := range n {
			for x := range n {
				c := mottle(x, y)
				pix = append(pix, c.R, c.G, c.B, c.A)
			}
		}
		m.img.WritePixels(pix)
	}
	return Painted{Albedo: m.img, Px: mottledPx, Reach: 96}
}

func TestTerrain_DrawsASheetOfTheAlbedoAtL3(t *testing.T) {
	gputest.Need(t)
	// in a light leaving a colour as it is, a pixel 8 world units, its middle a texel's
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0, 0, 1},
		"LightStrength": []float32{0}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{1, 1, 1}})
	draw := leveledDraw(t, &mottled{}, downward{cameratest.NewFromSpace(64, 64, 0)}, u)
	_, pix := draw(lod.L3)
	for y := range 64 {
		for x := range 64 {
			i := 4 * (y*64 + x)
			got := color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
			if want := mottle(x, y); !near(got, want, 2) {
				t.Fatalf("the sheet at the pixel (%d, %d) is %v, the albedo's %v", x, y, got, want)
			}
		}
	}
}

func near(a, b color.RGBA, by int) bool {
	d := func(x, y byte) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= by && d(a.G, b.G) <= by && d(a.B, b.B) <= by && d(a.A, b.A) <= by
}

// an aggregate 64 texels a side over the flat land, a texel a screen pixel looking downward: red
// at (20, 30), half blue at (40, 10)
func aggregated() (*render.Image, float32) {
	img := render.NewImage(64, 64)
	pix := make([]byte, 4*64*64)
	copy(pix[4*(30*64+20):], []byte{255, 0, 0, 255})
	copy(pix[4*(10*64+40):], []byte{0, 0, 128, 128})
	img.WritePixels(pix)
	return img, 1.0 / 8
}

func TestTerrain_LaysTheFiguresColourOverTheSheetAtL3Alone(t *testing.T) {
	gputest.Need(t)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0, 0, 1},
		"LightStrength": []float32{0}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{1, 1, 1}})
	cam := downward{cameratest.NewFromSpace(64, 64, 0)}
	img, perUnit := aggregated()
	plain := leveledDraw(t, &mottled{}, cam, u)
	summed := leveledDrawWith(t, &mottled{}, cam, u,
		func() (*render.Image, float32) { return img, perUnit })
	at := func(pix []byte, x, y int) color.RGBA {
		i := 4 * (y*64 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	for _, level := range []lod.Level{lod.L2, lod.L3} {
		_, without := plain(level)
		_, with := summed(level)
		for y := range 64 {
			for x := range 64 {
				want := at(without, x, y)
				if level == lod.L3 && x == 20 && y == 30 {
					want = color.RGBA{255, 0, 0, 255}
				}
				if level == lod.L3 && x == 40 && y == 10 {
					m := mottle(x, y)
					want = color.RGBA{m.R / 2, m.G / 2, m.B/2 + 128, 255}
				}
				if got := at(with, x, y); !near(got, want, 2) {
					t.Errorf("L%d: the pixel (%d, %d) is %v, want %v", level, x, y, got, want)
				}
			}
		}
	}
}

// leveledDrawWith is leveledDraw with the figures' colour summed over the sheet as aggregate
// gives it, none for nil
func leveledDrawWith(t *testing.T, surface Surface, cam camera.Camera, u render.Uniforms,
	aggregate func() (*render.Image, float32)) (draw func(level lod.Level) ([]span, []byte)) {
	t.Helper()
	grid := &atLevel{}
	v := view.NewSeen(cam, grid)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 512, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	system := view.NewSystem(space, &views, 512, 512)
	seen := func(camera.Camera) *view.View { return v }
	r := New(flatland{}, surface, noAir{}, Config{Seen: seen, Aggregate: aggregate})
	return func(level lod.Level) ([]span, []byte) {
		grid.level = level
		system.Update(nil, 0)
		screen := render.NewImage(64, 64)
		depth := render.NewDepth()
		screen.ClearDepth(depth)
		r.Draw(render.Target{Screen: screen, Depth: depth}, cam, u)
		pix := make([]byte, 4*64*64)
		screen.ReadPixels(pix)
		return r.drawn, pix
	}
}
