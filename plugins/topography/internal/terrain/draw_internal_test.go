package terrain

import (
	"image"
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
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
	if len(Shaders()) != cares {
		t.Fatalf("%d shaders, want one a care, %d", len(Shaders()), cares)
	}
	for care, s := range Shaders() {
		if err := s.Compile(); err != nil {
			t.Errorf("care %d: %v", care, err)
		}
	}
}

// level ground of 16 by 16 cells
type flatland struct{}

func (flatland) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return 17, 17, 32, make([]float32, 17*17), true
}
func (flatland) Version() uint64 { return 0 }

// grey ground with the sea on the cell (4, 10), painted blue, shining 0.9
type wetCell struct{ albedo, water *render.Image }

func (w *wetCell) Surface() Painted {
	if w.albedo == nil {
		w.albedo = render.NewImage(16, 16)
		w.albedo.Fill(color.RGBA{128, 128, 128, 255})
		w.albedo.SubImage(image.Rect(4, 10, 5, 11)).Fill(color.RGBA{30, 60, 200, 255})
		// the layers 4 pixels a cell in quadrants of 64: the shine layer's at the right
		w.water = render.NewImage(128, 128)
		w.water.Fill(color.RGBA{0, 0, 0, 255})
		w.water.SubImage(image.Rect(64+16, 40, 64+20, 44)).Fill(color.RGBA{0, 230, 255, 255})
	}
	wet := make([]bool, 16*16)
	wet[10*16+4] = true
	return Painted{Albedo: w.albedo, Px: 1, Water: w.water, WaterPx: 4, Reach: 96, Wet: wet,
		Wetness: 1}
}

// four sectors in a row, the first the whole world and the others empty beyond it; every point
// lies in the sector at
type farOff struct{ at int }

func (*farOff) Len() int  { return 4 }
func (*farOff) Cols() int { return 4 }
func (*farOff) Box(i int) geom.AABB {
	if i == 0 {
		return geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(512, 512))
	}
	return geom.NewAABB(geom.NewVec(512, 512), geom.NewVec(512, 512))
}
func (g *farOff) Of(float64, float64) int { return g.at }
func (*farOff) Off(int) bool              { return false }

type middled struct{ downward }

func (middled) Focus() (x, y, z float32) { return 256, 256, 0 }

func TestTerrain_KeepsTheWaterWetAndTheLandAlikeWithEveryCare(t *testing.T) {
	gputest.Need(t)
	cam := middled{downward{cameratest.NewFromSpace(64, 64, 0)}}
	grid := &farOff{}
	v := view.NewSeen(cam, grid)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 512, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 512, 512).Update(nil, 0)
	r := New(flatland{}, &wetCell{}, noAir{},
		Config{Seen: func(camera.Camera) *view.View { return v }})
	r.SetCare(1)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0.3, 0.2, 1},
		"LightStrength": []float32{1}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{0.2, 0.2, 0.2}, "Fog": []float32{0.5, 0.6, 0.9},
		"SkyOverhead": []float32{0.2, 0.4, 0.9}, "Toward": []float32{0, 0, 1},
		"Pixel": []float32{8}})
	var land [cares][]byte
	// the screen's middle in the sector 0, 2, 3, 4 and 5 rings from the one drawn (the grid 4
	// wide: 16 and 20 lie 4 and 5 rows down): the full care down to the least
	for care, at := range map[int]int{FullCare: 0, 3: 2, 2: 3, 1: 16, 0: 20} {
		grid.at = at
		screen := render.NewImage(64, 64)
		depth := render.NewDepth()
		screen.ClearDepth(depth)
		r.Draw(render.Target{Screen: screen, Depth: depth}, cam, u)
		if len(r.drawn) == 0 || r.drawn[0].first != 0 || r.drawn[0].care != care {
			t.Fatalf("runs %v, want the world's first with care %d", r.drawn, care)
		}
		pix := make([]byte, 4*64*64)
		screen.ReadPixels(pix)
		at := func(x, y int) []byte { return pix[4*(y*64+x) : 4*(y*64+x)+4] }
		// a pixel 8 world units: the wet cell spans (16..20, 40..44)
		if w := at(18, 42); w[3] != 255 || w[2] < w[0]+40 || w[2] < w[1]+40 {
			t.Errorf("care %d: the wet cell shows %v, want it opaque and blue", care, w)
		}
		land[care] = at(5, 5)
	}
	for care := range cares - 1 {
		if string(land[care]) != string(land[FullCare]) {
			t.Errorf("land with care %d shows %v, with full care %v: want alike without a baked "+
				"shade or a lamp", care, land[care], land[FullCare])
		}
	}
}
