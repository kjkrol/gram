package stick

import (
	"math"
	"testing"
)

var testOptics = optics{Cell: 32, TileW: 64, TileH: 32, HeightUnit: 2}.withDefaults()

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// the 2:1 view as drawn at its own zoom
func isometric(x, y, z float32) (float32, float32) {
	sx, sy := newOrtho(testOptics, eighth, testOptics.isoPitch()).Project(x, y, z)
	return sx * testOptics.tile(), sy * testOptics.tile()
}

func TestOrtho_IsometricallyACellIsADiamondAndHeightLiftsAPoint(t *testing.T) {
	if sx, sy := isometric(32, 0, 0); !near(sx, 32) || !near(sy, 16) {
		t.Errorf("one cell along x is drawn at (%v, %v), want (32, 16): down-right", sx, sy)
	}
	if sx, sy := isometric(0, 32, 0); !near(sx, -32) || !near(sy, 16) {
		t.Errorf("one cell along y is drawn at (%v, %v), want (-32, 16): down-left", sx, sy)
	}
	if _, sy := isometric(0, 0, 10); !near(sy, -20) {
		t.Errorf("a point 10 up is drawn at y %v, want -20: two units a height unit", sy)
	}
}

func TestOrtho_UnprojectInvertsProjectAndTowardLandsOnOneSpot(t *testing.T) {
	lower := testOptics
	lower.MinPitch = math.Pi / 18
	for _, turn := range []float64{0, 0.3, eighth, math.Pi / 2, 2, 4.5} {
		for _, pitch := range []float64{math.Pi / 12, lower.isoPitch(), 1.1, maxPitch} {
			p := newOrtho(lower, turn, pitch)
			for _, q := range [][3]float32{{100, 40, 0}, {13.5, 250, 7}, {-20, 5, 30}} {
				sx, sy := p.Project(q[0], q[1], q[2])
				if x, y := p.Unproject(sx, sy, q[2]); !near(x, q[0]) || !near(y, q[1]) {
					t.Errorf("turn %v, pitch %v: %v came back as (%v, %v)", turn, pitch, q, x, y)
				}
			}
			d := p.Toward()
			if !near(d[0]*d[0]+d[1]*d[1]+d[2]*d[2], 1) || d[2] <= 0 {
				t.Errorf("turn %v, pitch %v: Toward %v, want a unit vector rising", turn, pitch, d)
			}
			x0, y0 := p.Project(100, 50, 3)
			x1, y1 := p.Project(100+40*d[0], 50+40*d[1], 3+40*d[2])
			if !near(x0, x1) || !near(y0, y1) {
				t.Errorf("turn %v, pitch %v: 40 along Toward lands at (%v, %v), not on (%v, %v)",
					turn, pitch, x1, y1, x0, y0)
			}
		}
	}
}

func TestOrtho_LeaningDepthIsTheRowOfTheCellUnderThePoint(t *testing.T) {
	p := newOrtho(testOptics, eighth, testOptics.isoPitch())
	if back, front := p.Depth(0, 0, 0), p.Depth(32, 32, 0); back >= front {
		t.Errorf("depth at the origin %v is not behind (32, 32) %v", back, front)
	}
	if p.Depth(32, 0, 0) != p.Depth(0, 32, 0) {
		t.Error("two cells on the same row differ in depth")
	}
	tile, standing := p.Depth(48, 48, 0), p.Depth(40, 60, 30)
	if standing != tile || standing >= p.Depth(64, 48, 0) {
		t.Errorf("a thing anywhere in a cell has depth %v, want its tile's %v, before the next row",
			standing, tile)
	}
	d := p.Toward()
	if p.Depth(100+64*d[0], 50+64*d[1], 0) <= p.Depth(100, 50, 0) {
		t.Error("the way towards the eye is not nearer the eye")
	}
}

func TestOrtho_TurnedAQuarterTheWorldTurnsClockwiseOnTheScreen(t *testing.T) {
	p := newOrtho(testOptics, eighth+math.Pi/2, testOptics.isoPitch())
	s := testOptics.tile()
	if sx, sy := p.Project(32, 0, 0); !near(sx*s, -32) || !near(sy*s, 16) {
		t.Errorf("one cell along x is drawn at (%v, %v), want (-32, 16): where y ran before",
			sx*s, sy*s)
	}
	if sx, sy := p.Project(0, 32, 0); !near(sx*s, -32) || !near(sy*s, -16) {
		t.Errorf("one cell along y is drawn at (%v, %v), want (-32, -16): up-left", sx*s, sy*s)
	}
}

func TestOrtho_StraightDownItIsTheMapTurned(t *testing.T) {
	p := newOrtho(testOptics, 0, maxPitch)
	if sx, sy := p.Project(130, 70, 40); sx != 130 || sy != 70 {
		t.Errorf("unturned straight down (130, 70) 40 up is drawn at (%v, %v), want (130, 70)",
			sx, sy)
	}
	if p.Sorts() || p.Depth(130, 70, 40) != 70 || p.Toward() != [3]float32{0, 0, 1} {
		t.Errorf("straight down it sorts %v, depth %v, toward %v; want the map's: no, 70, up",
			p.Sorts(), p.Depth(130, 70, 40), p.Toward())
	}
	q := newOrtho(testOptics, math.Pi/2, maxPitch)
	if sx, sy := q.Project(32, 0, 5); !near(sx, 0) || !near(sy, 32) {
		t.Errorf("a quarter turned straight down x runs to (%v, %v), want (0, 32): down", sx, sy)
	}
	if _, low := newOrtho(testOptics, 0, math.Pi/4).Project(0, 32, 0); low >= 32 {
		t.Errorf("leaning, a cell runs %v down the screen, want less than straight down's 32", low)
	}
}
