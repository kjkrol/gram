package underground

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

func TestTubeShader_Compiles(t *testing.T) {
	gputest.Need(t)
	if err := tubeShader.Compile(); err != nil {
		t.Fatal(err)
	}
}

// straight down from 200 up, a screen pixel 8 world units
type downward struct{ camera.Camera }

func (downward) Rays() (camera.RayField, bool) {
	return camera.RayField{
		Origin: [3]float32{0, 0, 200},
		DX:     [3]float32{8, 0, 0},
		DY:     [3]float32{0, 8, 0},
		Dir:    [3]float32{0, 0, -1},
	}, true
}

func TestTubes_DrawAStretchWhereItLiesAndNothingBesideIt(t *testing.T) {
	gputest.Need(t)
	r := &tubes{}
	// from (100, 250) to (400, 250), 80 wide and 30 high, its walls along it standing
	r.bufs[0] = []float32{100, 250, 0, 40, 400, 250, 0, 30, wallLeft | wallRight, 100, 0, 0}
	screen := render.NewImage(64, 64)
	depth := render.NewDepth()
	screen.ClearDepth(depth)
	u := render.UniformsOf(map[string]any{"Ambience": []float32{0.5, 0.5, 0.5}})
	r.Draw(render.Target{Screen: screen, Depth: depth},
		downward{cameratest.NewFromSpace(64, 64, 0)}, u)
	pix := make([]byte, 4*64*64)
	screen.ReadPixels(pix)
	drawn := func(x, y int) bool { return pix[4*(y*64+x)+3] != 0 }
	for _, p := range [][2]int{{14, 28}, {30, 31}, {48, 34}} {
		if !drawn(p[0], p[1]) {
			t.Errorf("pixel %v on the stretch is not drawn", p)
		}
	}
	for _, p := range [][2]int{{5, 31}, {55, 31}, {30, 20}, {30, 42}} {
		if drawn(p[0], p[1]) {
			t.Errorf("pixel %v beside the stretch is drawn", p)
		}
	}
}
