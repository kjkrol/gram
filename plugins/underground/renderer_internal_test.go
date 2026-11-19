package underground

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
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

func TestStyle_GivesALookOnePlaceInTheTableWhateverKindsShareIt(t *testing.T) {
	r := &tubes{}
	mine := Look{Walls: color.RGBA{R: 90, A: 255}, Rough: true}
	r.style(1, "road", Look{})
	r.style(2, "shaft", mine)
	r.style(3, "drift", mine)
	if r.lookOf[1] != 0 || r.lookOf[2] != 1<<lookShift || r.lookOf[3] != r.lookOf[2] {
		t.Errorf("the road, the shaft and the drift at %v, %v and %v, want the road's first and "+
			"one place the next for both", r.lookOf[1], r.lookOf[2], r.lookOf[3])
	}
	walls, floor := r.table[lookFloats:lookFloats+4], r.table[lookFloats+4:lookFloats+8]
	if walls[0] != 90.0/255 || walls[3] != 1 {
		t.Errorf("the shaft's walls %v, want its own colour, rough", walls)
	}
	if floor[0] != float32(asphalt.R)/255 || floor[3] != 0 {
		t.Errorf("the shaft's floor %v, want the road's asphalt without its lines", floor)
	}
	defer func() {
		if recover() == nil {
			t.Error("a look past the table's places found one")
		}
	}()
	for k := range maxLooks {
		r.style(kind.ID(10+k), "more", Look{Walls: color.RGBA{G: uint8(k + 1), A: 255}})
	}
}

func TestTubes_DrawAStretchInTheLookOfItsKind(t *testing.T) {
	gputest.Need(t)
	r := &tubes{}
	red := color.RGBA{R: 200, G: 30, B: 20, A: 255}
	r.style(1, "red", Look{Walls: red, Floor: red, Roof: red})
	look := r.lookOf[1]
	r.bufs[0] = []float32{100, 250, 0, 40, 400, 250, 0, 30, wallLeft | wallRight + look, 100, 0, 0}
	screen := render.NewImage(64, 64)
	depth := render.NewDepth()
	screen.ClearDepth(depth)
	u := render.UniformsOf(map[string]any{"Ambience": []float32{0.5, 0.5, 0.5}})
	r.Draw(render.Target{Screen: screen, Depth: depth},
		downward{cameratest.NewFromSpace(64, 64, 0)}, u)
	pix := make([]byte, 4*64*64)
	screen.ReadPixels(pix)
	if p := pix[4*(31*64+30):][:3]; p[0] <= 2*p[1] || p[0] <= 2*p[2] {
		t.Errorf("the stretch drawn %v, want the red of its kind", p)
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
	got := frame(nil, geom.NewVec(50, 20), geom.NewVec(1, 0), 10, 0.5, 16, f, 0)
	if len(got) != 3*tubeFloats {
		t.Fatalf("a frame of %d floats, want two jambs and a lintel", len(got))
	}
	for k := 0; k < len(got); k += tubeFloats {
		if p := got[k : k+tubeFloats]; int(p[8])&framed == 0 || p[0] <= 50 || p[4] != 34 {
			t.Errorf("the piece %v, want a frame's from outside the mouth at x 50 to 16 in", p)
		}
	}
	if l, r := got[1], got[tubeFloats+1]; f32.Abs(l-20) <= 3 || f32.Abs(r-20) <= 3 || l == r {
		t.Errorf("the jambs at y %v and %v, want one either side of a tube 3 off its middle", l, r)
	}
	if lintel := got[2*tubeFloats:]; lintel[2] < 14 || lintel[2] > 14.1 || lintel[6] != lintel[2] ||
		lintel[3] <= 3 ||
		lintel[2]+lintel[7] <= 22 {
		t.Errorf(
			"the lintel %v, want it level from just over the roof at 14 to over the roof 16 in, "+
				"at 22, wider than the tube",
			lintel,
		)
	}
	sunk := frame(nil, geom.NewVec(50, 20), geom.NewVec(1, 0), 10, -0.5, 16, f, 0)
	if jamb := sunk[:tubeFloats]; jamb[2] != 2 || jamb[2]+jamb[7] <= 14 {
		t.Errorf("the jamb %v of a floor falling to 2, want it from 2 to over the roof at 14",
			jamb)
	}
}

func TestBent_LaysATurnInPiecesMeetingWallToWallAndSquareAtItsEdges(t *testing.T) {
	f := cell.Floor{Width: 6, Clear: 4}
	m := geom.NewVec(15, 15)
	out := func(x, y float32) leg {
		l := f32.Hypot(x, y)
		return leg{u: geom.NewVec(x/l, y/l), long: 10 * l, far: 2,
			end: geom.NewVec(m.X+5*x, m.Y+5*y)}
	}
	for name, legs := range map[string][2]leg{
		"a square turn": {out(-1, 0), out(0, 1)},
		"half a turn":   {out(-1, 0), out(1, 1)},
	} {
		a, b := legs[0], legs[1]
		got := bent(nil, a, b, m, 3, f, 0)
		if len(got) != bends*tubeFloats {
			t.Fatalf("%s: %d floats, want %d pieces", name, len(got), bends)
		}
		type wall struct{ begin, end [2]geom.Vec } // left, right
		walls := make([]wall, bends)
		for k := range bends {
			p := got[k*tubeFloats : (k+1)*tubeFloats]
			pa, pb := geom.NewVec(p[0], p[1]), geom.NewVec(p[4], p[5])
			l := f32.Hypot(pb.X-pa.X, pb.Y-pa.Y)
			d := geom.NewVec((pb.X-pa.X)/l, (pb.Y-pa.Y)/l)
			n := geom.NewVec(-d.Y*p[3], d.X*p[3])
			end, begin := p[10], p[11]
			for i, side := range [2]float32{1, -1} {
				walls[k].begin[i] = geom.NewVec(pa.X+side*(n.X+d.X*begin),
					pa.Y+side*(n.Y+d.Y*begin))
				walls[k].end[i] = geom.NewVec(pb.X-side*(-n.X+d.X*end),
					pb.Y-side*(-n.Y+d.Y*end))
			}
			if k > 0 {
				q := got[(k-1)*tubeFloats : k*tubeFloats]
				if q[4] != p[0] || q[5] != p[1] || q[6] != p[2] {
					t.Errorf("%s: piece %d begins at %v, the one before ends at %v", name, k,
						p[:3], q[4:7])
				}
				if f32.Abs(q[9]+f32.Hypot(q[4]-q[0], q[5]-q[1])-p[9]) > 1e-3 {
					t.Errorf("%s: the lamps counted %v on into piece %d, from %v", name, p[9],
						k, q[9])
				}
				for i := range 2 {
					w, v := walls[k-1].end[i], walls[k].begin[i]
					if f32.Hypot(w.X-v.X, w.Y-v.Y) > 1e-3 {
						t.Errorf("%s: a wall of piece %d ends at %v, the next begins at %v",
							name, k-1, w, v)
					}
				}
			}
		}
		if got[0] != a.end.X || got[1] != a.end.Y || got[len(got)-8] != b.end.X ||
			got[len(got)-7] != b.end.Y {
			t.Errorf("%s: the turn runs from %v to %v, want from %v to %v", name, got[:2],
				got[len(got)-8:len(got)-6], a.end, b.end)
		}
		for i := range 2 {
			at, to := walls[0].begin[i], walls[bends-1].end[i]
			if s := (at.X-a.end.X)*a.u.X + (at.Y-a.end.Y)*a.u.Y; f32.Abs(s) > 1e-3 {
				t.Errorf("%s: a wall begins %v off the edge it comes in by, want square on it",
					name, s)
			}
			if s := (to.X-b.end.X)*b.u.X + (to.Y-b.end.Y)*b.u.Y; f32.Abs(s) > 1e-3 {
				t.Errorf("%s: a wall ends %v off the edge it goes out by, want square on it",
					name, s)
			}
		}
	}
}
