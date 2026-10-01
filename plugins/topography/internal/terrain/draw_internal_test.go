package terrain

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// bumped is level ground 16 cells a side with one corner, (4, 10), raised 40.
type bumped struct{}

func (bumped) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	heights = make([]float32, 17*17)
	heights[10*17+4] = 40
	return 17, 17, 32, heights, true
}
func (bumped) Version() uint64 { return 0 }

// grey is the board painted one grey.
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

// downward looks straight down at the ground from 200 up, a screen pixel 8 world units.
type downward struct{ camera.Camera }

func (downward) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{0, 0, 200}, DX: [3]float32{8, 0, 0}, DY: [3]float32{0, 8, 0}, Dir: [3]float32{0, 0, -1}}, true
}

// Drawn from above under a high sun, level ground is lit alike everywhere and the raised corner
// shows where it stands, its slopes lit otherwise: the lattice's quadrants each read as written.
func TestTerrain_DrawsTheLevelEvenlyAndABumpWhereItStands(t *testing.T) {
	needGPU(t)
	r := New(bumped{}, &grey{}, noAir{}, Config{})
	screen := render.NewImage(64, 64)
	u := render.UniformsOf(map[string]any{"Sun": []float32{0.3, 0.2, 1}, "SunStrength": []float32{1}, "SunColor": []float32{1, 1, 1}, "Ambience": []float32{0.2, 0.2, 0.2}})
	depth := render.NewDepth()
	screen.ClearDepth(depth)
	r.Draw(render.Target{Screen: screen, Depth: depth}, downward{icamera.NewFromSpace(64, 64, 0)}, u)
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

// The ground's mesh shader compiles on the composer's library, the water among the materials it
// calls.
func TestTerrainShader_Compiles(t *testing.T) {
	needGPU(t)
	if err := Shader().Compile(); err != nil {
		t.Fatal(err)
	}
}

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
