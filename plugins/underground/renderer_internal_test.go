package underground

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"

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

func TestMitre_CutsAStretchOnThePlaneBetweenItAndTheOneItTurnsInto(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, 10)
	way := func(dx, dy int) int { d, _ := grid.Direction(g, dx, dy); return d }
	east, south, west, se := way(1, 0), way(0, 1), way(-1, 0), way(1, 1)
	r := &tubes{}
	for name, tc := range map[string]struct {
		in, other int
		want      float32
	}{
		// coming in from the east its left is the north, the outer side: 3 past the middle
		"a square turn, the outer side on the left":  {east, south, -3},
		"a square turn, the outer side on the right": {south, east, 3},
		"straight on": {east, west, 0},
		"half a turn": {west, se, 3 * 0.41421356},
	} {
		f := cell.Floor{Width: 6, Links: 1<<tc.in | 1<<tc.other}
		if got := r.mitre(g, f, tc.in); f32.Abs(got-tc.want) > 1e-4 {
			t.Errorf("%s: a mitre of %v, want %v", name, got, tc.want)
		}
	}
}

func TestFrame_StandsRoundTheMouthLevelOverTheRoofAsItRunsIn(t *testing.T) {
	f := cell.Floor{Width: 6, Clear: 4}
	// the floor at 10 at the mouth, climbing a half a unit, the frame running 16 in
	got := frame(nil, geom.NewVec(50, 20), geom.NewVec(1, 0), 10, 0.5, 16, f)
	if len(got) != 3*tubeFloats {
		t.Fatalf("a frame of %d floats, want two jambs and a lintel", len(got))
	}
	for k := 0; k < len(got); k += tubeFloats {
		if p := got[k : k+tubeFloats]; p[11] != 1 || p[0] <= 50 || p[4] != 34 {
			t.Errorf("the piece %v, want a frame's from outside the mouth at x 50 to 16 in", p)
		}
	}
	if l, r := got[1], got[tubeFloats+1]; f32.Abs(l-20) <= 3 || f32.Abs(r-20) <= 3 || l == r {
		t.Errorf("the jambs at y %v and %v, want one either side of a tube 3 off its middle", l, r)
	}
	if lintel := got[2*tubeFloats:]; lintel[2] < 14 || lintel[2] > 14.1 || lintel[6] != lintel[2] || lintel[3] <= 3 ||
		lintel[2]+lintel[7] <= 22 {
		t.Errorf("the lintel %v, want it level from just over the roof at 14 to over the roof 16 in, "+
			"at 22, wider than the tube", lintel)
	}
	sunk := frame(nil, geom.NewVec(50, 20), geom.NewVec(1, 0), 10, -0.5, 16, f)
	if jamb := sunk[:tubeFloats]; jamb[2] != 2 || jamb[2]+jamb[7] <= 14 {
		t.Errorf("the jamb %v of a floor falling to 2, want it from 2 to over the roof at 14",
			jamb)
	}
}
