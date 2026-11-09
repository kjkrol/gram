package terrain

import (
	"image"
	"image/color"
	"math"
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
	if want := int(lod.Omitted - lod.Standard); len(Shaders()) != want {
		t.Fatalf("%d shaders, want one a level drawn from Standard, %d", len(Shaders()), want)
	}
	for k, s := range Shaders() {
		if err := s.Compile(); err != nil {
			t.Errorf("L%d: %v", lod.Standard+lod.Level(k), err)
		}
	}
	for k, s := range photoShaders {
		if err := s.Compile(); err != nil {
			t.Errorf("the sheets at L%d: %v", lod.Strategic+lod.Level(k), err)
		}
		if err := topsShaders[k].Compile(); err != nil {
			t.Errorf("the sheets' tops at L%d: %v", lod.Strategic+lod.Level(k), err)
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
	var land, sea [lod.L6]color.RGBA
	for level := lod.L2; level < lod.L6; level++ {
		runs, pix := draw(level)
		// at L5 the world's coarse run and the skirt, which follows it, are one
		if n := len(runs); runs[0].level != level || runs[n-1].level != lod.L5 ||
			n != 2 && level != lod.L5 {
			t.Fatalf("runs %v, want the world's at L%d and the skirt's at L5", runs, level)
		}
		land[level], sea[level] = at(pix, 5, 5), at(pix, 40, 40)
		if land[level].A != 255 || sea[level].A != 255 {
			t.Fatalf("L%d: the land %v and the sea %v, want both drawn", level, land[level],
				sea[level])
		}
	}
	for level := lod.L3; level < lod.L6; level++ {
		if !near(land[level], land[lod.L2], 1) {
			t.Errorf("the land at L%d is %v, at L2 %v: want alike without a baked shade, a "+
				"lamp, a grain or a grid", level, land[level], land[lod.L2])
		}
		if near(sea[level], sea[level-1], 2) {
			t.Errorf("the sea at L%d is %v, as at L%d: want it without what L%d drops", level,
				sea[level], level-1, level)
		}
	}
	// the sheet: the sea's albedo in the light the land is in, no water
	light := float32(land[lod.L5].R) / float32(landColour.R)
	in := func(v byte) byte { return byte(float32(v)*light + 0.5) }
	want := color.RGBA{in(seaColour.R), in(seaColour.G), in(seaColour.B), 255}
	if !near(sea[lod.L5], want, 3) {
		t.Errorf("the sea as a sheet is %v, want the albedo's %v in the light", sea[lod.L5], want)
	}
	runs, pix := draw(lod.L6)
	if len(runs) != 1 || runs[0].level != lod.L5 {
		t.Errorf("runs at L6 %v, want the skirt alone, as a sheet", runs)
	}
	for k := 3; k < len(pix); k += 4 {
		if pix[k] != 0 {
			t.Fatalf("the pixel %d is drawn at L6, want nothing", k/4)
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

func TestTerrain_DrawsTheGridAtL2Alone(t *testing.T) {
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
	for level := lod.L2; level < lod.L6; level++ {
		_, pix := draw(level)
		edge, inside := at(pix, 16, 5), at(pix, 8, 5)
		if want := level == lod.L2; near(edge, red, 2) != want || near(inside, red, 2) {
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

func TestTerrain_DrawsASheetOfTheAlbedoAtL5(t *testing.T) {
	gputest.Need(t)
	// in a light leaving a colour as it is, a pixel 8 world units, its middle a texel's
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0, 0, 1},
		"LightStrength": []float32{0}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{1, 1, 1}})
	draw := leveledDraw(t, &mottled{}, downward{cameratest.NewFromSpace(64, 64, 0)}, u)
	_, pix := draw(lod.L5)
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

// ground striped down each cell, Px pixels a cell, its left half light and its right dark, the
// green rising row by row, with its half and quarter as the painter shrinks it
type striped struct{ albedo, half, quarter *render.Image }

const stripedPx = 4

func stripe(x, y int) color.RGBA {
	v := byte(150)
	if x%stripedPx >= stripedPx/2 {
		v = 70
	}
	return color.RGBA{v, byte(60 + 8*(y/stripedPx)), 220 - v, 255}
}

func (s *striped) Surface() Painted {
	if s.albedo == nil {
		n := 16 * stripedPx
		pix := make([]byte, 0, 4*n*n)
		for y := range n {
			for x := range n {
				c := stripe(x, y)
				pix = append(pix, c.R, c.G, c.B, c.A)
			}
		}
		s.albedo = imageOf(pix, n)
		s.half = imageOf(meansOf(pix, n, 2), n/2)
		s.quarter = imageOf(meansOf(pix, n, 4), n/4)
	}
	return Painted{Albedo: s.albedo, AlbedoHalf: s.half, AlbedoQuarter: s.quarter, Px: stripedPx,
		Reach: 96}
}

func imageOf(pix []byte, n int) *render.Image {
	img := render.NewImage(n, n)
	img.WritePixels(pix)
	return img
}

// the n by n pixels of pix shrunk by k, each the mean of the k by k under it
func meansOf(pix []byte, n, k int) []byte {
	var out []byte
	for y := 0; y < n; y += k {
		for x := 0; x < n; x += k {
			for c := range 4 {
				sum := 0
				for dy := range k {
					for dx := range k {
						sum += int(pix[4*((y+dy)*n+x+dx)+c])
					}
				}
				out = append(out, byte((sum+k*k/2)/(k*k)))
			}
		}
	}
	return out
}

func TestTerrain_DrawsTheAlbedoShrunkAtL4AndL5SoACellsPatternFades(t *testing.T) {
	gputest.Need(t)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0, 0, 1},
		"LightStrength": []float32{0}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{1, 1, 1}})
	draw := leveledDraw(t, &striped{}, overSea{cameratest.NewFromSpace(64, 64, 0)}, u)
	// a pixel 2 world units, a cell 16 pixels: the row 8 runs along the middle of the cells'
	// row 8, four cells across
	const row, cells = 8, 4
	var energy [lod.L6]float64
	for _, level := range []lod.Level{lod.L3, lod.L4, lod.L5} {
		_, pix := draw(level)
		var re, im float64
		for x := range 64 {
			v := float64(pix[4*(row*64+x)])
			a := 2 * math.Pi * cells * float64(x) / 64
			re += v * math.Cos(a)
			im -= v * math.Sin(a)
		}
		energy[level] = re*re + im*im
		if level != lod.L5 {
			continue
		}
		for x := range 64 {
			i := 4 * (row*64 + x)
			got := color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
			if want := (color.RGBA{110, 60 + 8*8, 110, 255}); !near(got, want, 2) {
				t.Fatalf("L5: the pixel (%d, %d) is %v, want the cell's mean %v", x, row, got, want)
			}
		}
	}
	if !(4*energy[lod.L5] <= energy[lod.L3] && energy[lod.L5] < energy[lod.L4] &&
		energy[lod.L4] < energy[lod.L3]) {
		t.Errorf("the energy at a cell's period along a row L3 %.0f, L4 %.0f, L5 %.0f: want it "+
			"falling, at L5 a quarter of L3's at most", energy[lod.L3], energy[lod.L4],
			energy[lod.L5])
	}
}

func near(a, b color.RGBA, by int) bool {
	d := func(x, y byte) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= by && d(a.G, b.G) <= by && d(a.B, b.B) <= by && d(a.A, b.A) <= by
}

// the sheets of what stands over the flat land, a world unit over it: at L4 64 texels a side, a
// texel a screen pixel looking downward, red at (20, 30) and half blue at (40, 10); at L5 half
// blue all over
func sheeted() (*render.Image, *render.Image, *render.Image, *render.Image, float32) {
	albedo := render.NewImage(64, 64)
	pix := make([]byte, 4*64*64)
	copy(pix[4*(30*64+20):], []byte{255, 0, 0, 255})
	copy(pix[4*(10*64+40):], []byte{0, 0, 128, 128})
	albedo.WritePixels(pix)
	half := render.NewImage(32, 32)
	pix = make([]byte, 4*32*32)
	for i := 0; i < len(pix); i += 4 {
		pix[i+2], pix[i+3] = 128, 128
	}
	half.WritePixels(pix)
	level := func(int, int) (float32, float32, float32) { return 1, 0, 0 }
	return albedo, reliefOf(64, 64, level), half, reliefOf(32, 32, level), 1.0 / 8
}

// reliefOf is a sheet's relief w by h texels, each as at says: its top's height and the x and y
// of the way it faces
func reliefOf(w, h int, at func(x, y int) (top, nx, ny float32)) *render.Image {
	img := render.NewImage(w, h)
	pix := make([]byte, 0, 4*w*h)
	for y := range h {
		for x := range w {
			top, nx, ny := at(x, y)
			f := half(top)
			pix = append(pix, byte(f), byte(f>>8), byte((nx+1)/2*255+0.5), byte((ny+1)/2*255+0.5))
		}
	}
	img.WritePixels(pix)
	return img
}

// half is v a float16, rounded down; v a whole number under 2048 or zero is exact
func half(v float32) uint16 {
	if v == 0 {
		return 0
	}
	bits := math.Float32bits(v)
	sign := uint16(bits>>16) & 0x8000
	exp := int(bits>>23&0xff) - 127 + 15
	return sign | uint16(exp)<<10 | uint16(bits>>13&0x3ff)
}

func TestTerrain_LaysTheSheetOverTheGroundAtL4AndL5(t *testing.T) {
	gputest.Need(t)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0, 0, 1},
		"LightStrength": []float32{0}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{1, 1, 1}})
	cam := downward{cameratest.NewFromSpace(64, 64, 0)}
	plain := leveledDraw(t, &mottled{}, cam, u)
	laid := leveledDrawWith(t, &mottled{}, cam, u, sheeted)
	at := func(pix []byte, x, y int) color.RGBA {
		i := 4 * (y*64 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	halfBlue := func(c color.RGBA) color.RGBA {
		return color.RGBA{c.R / 2, c.G / 2, c.B/2 + 128, 255}
	}
	for _, level := range []lod.Level{lod.L3, lod.L4, lod.L5} {
		_, without := plain(level)
		_, with := laid(level)
		for y := range 64 {
			for x := range 64 {
				want := at(without, x, y)
				switch {
				case level == lod.L5:
					want = halfBlue(want)
				case level == lod.L4 && x == 20 && y == 30:
					want = color.RGBA{255, 0, 0, 255}
				case level == lod.L4 && x == 40 && y == 10:
					want = halfBlue(want)
				}
				if got := at(with, x, y); !near(got, want, 2) {
					t.Fatalf("L%d: the pixel (%d, %d) is %v, want %v", level, x, y, got, want)
				}
			}
		}
	}
}

// sheets gives the sheets of what stands, as the world's Read.Sheet
type sheets = func() (_, _, _, _ *render.Image, _ float32)

// leveledDrawWith is leveledDraw with the sheets of what stands laid over the ground as sheet
// gives them, none for nil
func leveledDrawWith(t *testing.T, surface Surface, cam camera.Camera, u render.Uniforms,
	sheet sheets) (draw func(level lod.Level) ([]span, []byte)) {
	t.Helper()
	return paintedDraw(t, surface, cam, u, sheet, func(int) uint32 { return 1 })
}

// paintedDraw is leveledDrawWith with the version of the photograph the sheets hold as painted
// says
func paintedDraw(t *testing.T, surface Surface, cam camera.Camera, u render.Uniforms,
	sheet sheets, painted func(int) uint32) (draw func(level lod.Level) ([]span, []byte)) {
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
	cfg := Config{Seen: seen, Sheet: sheet}
	if sheet != nil {
		cfg.Painted = painted
	}
	r := New(flatland{}, surface, noAir{}, cfg)
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

// domed is the sheets of what stands over the flat land, 8 world units a texel: a white dome
// round the texel (32, 32), 8 texels across its middle, top high in it; the same halved at L5
func domed(top float32) sheets {
	disc := func(x, y, r int) bool { return (x-32)*(x-32)+(y-32)*(y-32) <= r*r }
	albedo, half := render.NewImage(64, 64), render.NewImage(32, 32)
	pix := make([]byte, 0, 4*64*64)
	for y := range 64 {
		for x := range 64 {
			if disc(x, y, 8) {
				pix = append(pix, 255, 255, 255, 255)
			} else {
				pix = append(pix, 0, 0, 0, 0)
			}
		}
	}
	albedo.WritePixels(pix)
	pix = pix[:0]
	for y := range 32 {
		for x := range 32 {
			if disc(2*x, 2*y, 8) {
				pix = append(pix, 255, 255, 255, 255)
			} else {
				pix = append(pix, 0, 0, 0, 0)
			}
		}
	}
	half.WritePixels(pix)
	// the dome top·(1 − r²/2), r a share of its half-width, facing away from its middle
	dome := func(scale int) func(x, y int) (float32, float32, float32) {
		return func(x, y int) (float32, float32, float32) {
			dx, dy := float32(x*scale-32)/8, float32(y*scale-32)/8
			if dx*dx+dy*dy > 1 {
				return 0, 0, 0
			}
			gx, gy := -top*dx/64, -top*dy/64
			l := float32(math.Sqrt(float64(gx*gx + gy*gy + 1)))
			return top * (1 - (dx*dx+dy*dy)/2), -gx / l, -gy / l
		}
	}
	return func() (*render.Image, *render.Image, *render.Image, *render.Image, float32) {
		return albedo, reliefOf(64, 64, dome(1)), half, reliefOf(32, 32, dome(2)), 1.0 / 8
	}
}

func TestTerrain_LightsTheSheetsDomeOnTheSideTowardTheSun(t *testing.T) {
	gputest.Need(t)
	cam := downward{cameratest.NewFromSpace(64, 64, 0)}
	lit := func(way float32) []byte {
		u := render.UniformsOf(map[string]any{"LightWay": []float32{way, 0, 0.6},
			"LightStrength": []float32{1}, "LightColor": []float32{1, 1, 1},
			"Ambience": []float32{0.2, 0.2, 0.2}})
		_, pix := leveledDrawWith(t, &grey{}, cam, u, domed(64))(lod.L4)
		return pix
	}
	east, west := lit(0.8), lit(-0.8)
	red := func(pix []byte, x, y int) byte { return pix[4*(y*64+x)] }
	if e, w := red(east, 36, 32), red(east, 28, 32); e <= w+20 {
		t.Errorf("the sun in the east lights the dome's east side %d, its west %d: want brighter",
			e, w)
	}
	if e, w := red(west, 36, 32), red(west, 28, 32); w <= e+20 {
		t.Errorf("the sun in the west lights the dome's west side %d, its east %d: want brighter",
			w, e)
	}
	if g := red(east, 5, 5); g == red(east, 32, 32) {
		t.Errorf("the ground far off is %d as the dome's middle, want the dome drawn over it", g)
	}
}

// slanted looks north and down at 37° over the flat land, a screen pixel 8 world units across
// and 10 along the ground down the screen at its level, the top row seeing y = 600 there
type slanted struct{ camera.Camera }

func (slanted) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{0, 300, 400}, DX: [3]float32{8, 0, 0},
		DY: [3]float32{0, -6.4, -4.8}, Dir: [3]float32{0, 0.6, -0.8}}, true
}

func TestTerrain_StandsTheSheetsTopsOverTheGround(t *testing.T) {
	gputest.Need(t)
	cam := slanted{cameratest.NewFromSpace(64, 64, 0)}
	row := func(top float32) float32 {
		_, pix := leveledDrawWith(t, &grey{}, cam, unlit(), domed(top))(lod.L4)
		return whiteRow(t, pix)
	}
	// the middle at 64 lies 0.75·64/10 rows nearer the top than on the ground, its rim not at all
	if low, high := row(1), row(64); low-high < 1.5 {
		t.Errorf("the dome's white at row %v standing 64 high, %v lying on the ground: want it "+
			"higher on the screen", high, low)
	}
}

func TestTerrain_StandsTheSheetsTopsAnewAsItsPhotographChanges(t *testing.T) {
	gputest.Need(t)
	cam := slanted{cameratest.NewFromSpace(64, 64, 0)}
	low, high := domed(1), domed(64)
	shown, version := low, uint32(1)
	draw := paintedDraw(t, &grey{}, cam, unlit(),
		func() (_, _, _, _ *render.Image, _ float32) { return shown() },
		func(int) uint32 { return version })
	_, pix := draw(lod.L4)
	lying := whiteRow(t, pix)
	shown = high
	if _, pix = draw(lod.L4); whiteRow(t, pix) != lying {
		t.Errorf("the dome's white at row %v with the photograph's version kept, %v before: "+
			"want the tops worked out once", whiteRow(t, pix), lying)
	}
	version++
	if _, pix = draw(lod.L4); lying-whiteRow(t, pix) < 1.5 {
		t.Errorf("the dome's white at row %v in a photograph anew standing 64 high, %v lying on "+
			"the ground: want it higher on the screen", whiteRow(t, pix), lying)
	}
}

// unlit is the frame's uniforms with nothing but an ambient white light
func unlit() render.Uniforms {
	return render.UniformsOf(map[string]any{"LightWay": []float32{0, 0, 1},
		"LightStrength": []float32{0}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{1, 1, 1}})
}

// whiteRow is the row the white of a dome lies at on the screen pix, 64 pixels a side, on the mean
func whiteRow(t *testing.T, pix []byte) float32 {
	t.Helper()
	var sum, n float32
	for y := range 64 {
		for x := range 64 {
			if pix[4*(y*64+x)] > 200 {
				sum, n = sum+float32(y), n+1
			}
		}
	}
	if n == 0 {
		t.Fatal("no dome seen")
	}
	return sum / n
}
