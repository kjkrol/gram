package cameras

import (
	"math"
	"testing"
)

func TestProjection_ACellIsADiamondAndHeightLiftsAPoint(t *testing.T) {
	if sx, sy := testProjection.Project(32, 0, 0); !near(sx, 32) || !near(sy, 16) {
		t.Errorf("one cell along x projects to (%v, %v), want (32, 16): down-right", sx, sy)
	}
	if sx, sy := testProjection.Project(0, 32, 0); !near(sx, -32) || !near(sy, 16) {
		t.Errorf("one cell along y projects to (%v, %v), want (-32, 16): down-left", sx, sy)
	}
	if _, sy := testProjection.Project(0, 0, 10); !near(sy, -20) {
		t.Errorf("a point 10 up projects to y %v, want -20: two units a height unit", sy)
	}
}

func TestProjection_UnprojectInvertsProject(t *testing.T) {
	for _, p := range [][3]float32{{0, 0, 0}, {100, 40, 0}, {13.5, 250, 7}, {-20, 5, 30}} {
		sx, sy := testProjection.Project(p[0], p[1], p[2])
		x, y := testProjection.Unproject(sx, sy, p[2])
		if !near(x, p[0]) || !near(y, p[1]) {
			t.Errorf("point %v went to (%v, %v) and came back as (%v, %v)", p, sx, sy, x, y)
		}
	}
}

func TestProjection_DepthIsTheRowOfTheCellUnderThePoint(t *testing.T) {
	back, front := testProjection.Depth(0, 0, 0), testProjection.Depth(32, 32, 0)
	if back >= front {
		t.Errorf("depth at the origin %v is not behind (32, 32) %v", back, front)
	}
	if testProjection.Depth(32, 0, 0) != testProjection.Depth(0, 32, 0) {
		t.Error("two cells on the same row differ in depth")
	}
	if tile, standing := testProjection.Depth(48, 48, 0), testProjection.Depth(40, 60, 30); standing != tile || standing >= testProjection.Depth(64, 48, 0) {
		t.Errorf("a thing anywhere in a cell has depth %v, want its tile's %v, before the next row", standing, tile)
	}
}

func TestProjection_TowardIsWhereEveryPointOnItLandsOnOneSpot(t *testing.T) {
	p := testProjection.withDefaults()
	d := p.Toward()
	if !near(d[0]*d[0]+d[1]*d[1]+d[2]*d[2], 1) || d[2] <= 0 {
		t.Fatalf("Toward %v, want a unit vector rising towards the eye", d)
	}
	x0, y0 := p.Project(100, 50, 3)
	x1, y1 := p.Project(100+40*d[0], 50+40*d[1], 3+40*d[2])
	if !near(x0, x1) || !near(y0, y1) {
		t.Errorf("40 along Toward lands at (%v, %v), not on (%v, %v)", x1, y1, x0, y0)
	}
	if p.Depth(100+40*d[0], 50+40*d[1], 0) <= p.Depth(100, 50, 0) {
		t.Error("the way towards the eye is not nearer the eye")
	}
}

func TestProjection_TurnedItStillInvertsAndLooksAlongToward(t *testing.T) {
	for _, heading := range []float32{0.3, math.Pi / 2, 2, 4.5} {
		p := testProjection.turned(heading)
		for _, q := range [][3]float32{{100, 40, 0}, {13.5, 250, 7}} {
			sx, sy := p.Project(q[0], q[1], q[2])
			if x, y := p.Unproject(sx, sy, q[2]); !near(x, q[0]) || !near(y, q[1]) {
				t.Errorf("turned %v: point %v came back as (%v, %v)", heading, q, x, y)
			}
		}
		d := p.Toward()
		x0, y0 := p.Project(100, 50, 3)
		if x1, y1 := p.Project(100+40*d[0], 50+40*d[1], 3+40*d[2]); !near(x0, x1) || !near(y0, y1) {
			t.Errorf("turned %v: 40 along Toward lands at (%v, %v), not on (%v, %v)", heading, x1, y1, x0, y0)
		}
		if p.Depth(100+64*d[0], 50+64*d[1], 0) <= p.Depth(100, 50, 0) {
			t.Errorf("turned %v: the way towards the eye is not nearer the eye", heading)
		}
		if p.Depth(40, 60, 0) != p.Depth(63, 33, 20) {
			t.Errorf("turned %v: two points of one cell differ in depth", heading)
		}
	}
}

func TestProjection_TurnedAQuarterTheWorldTurnsClockwiseOnTheScreen(t *testing.T) {
	p := testProjection.turned(math.Pi / 2)
	if sx, sy := p.Project(32, 0, 0); !near(sx, -32) || !near(sy, 16) {
		t.Errorf("one cell along x projects to (%v, %v), want (-32, 16): where y ran before", sx, sy)
	}
	if sx, sy := p.Project(0, 32, 0); !near(sx, -32) || !near(sy, -16) {
		t.Errorf("one cell along y projects to (%v, %v), want (-32, -16): up-left", sx, sy)
	}
	if h := testProjection.turned(-math.Pi / 2).Heading; !near(h, 3*math.Pi/2) {
		t.Errorf("turned back a quarter the heading is %v, want 3π/2", h)
	}
}

func TestProjection_TiltedItStillInvertsLooksAlongTowardAndFlattensTheGroundLow(t *testing.T) {
	flat := testProjection.Pitch
	if !near(flat, float32(math.Asin(0.5))) {
		t.Fatalf("the 2:1 view looks down at %v, want 30°", flat)
	}
	_, row := testProjection.Project(32, 32, 0)
	// lower lets the eye down to ten degrees, where the default floor of 30° would hold it
	lower := testProjection
	lower.MinPitch = math.Pi / 18
	for _, pitch := range []float32{math.Pi / 12, math.Pi / 2, 1.1} {
		p := lower.turned(0.7).tilted(pitch)
		for _, q := range [][3]float32{{100, 40, 0}, {13.5, 250, 7}} {
			sx, sy := p.Project(q[0], q[1], q[2])
			if x, y := p.Unproject(sx, sy, q[2]); !near(x, q[0]) || !near(y, q[1]) {
				t.Errorf("pitch %v: point %v came back as (%v, %v)", pitch, q, x, y)
			}
		}
		d := p.Toward()
		if !near(d[0]*d[0]+d[1]*d[1]+d[2]*d[2], 1) {
			t.Errorf("pitch %v: Toward %v is no unit vector", pitch, d)
		}
		x0, y0 := p.Project(100, 50, 3)
		if x1, y1 := p.Project(100+40*d[0], 50+40*d[1], 3+40*d[2]); !near(x0, x1) || !near(y0, y1) {
			t.Errorf("pitch %v: 40 along Toward lands at (%v, %v), not on (%v, %v)", pitch, x1, y1, x0, y0)
		}
	}
	if _, low := lower.tilted(math.Pi/12).Project(32, 32, 0); low >= row {
		t.Errorf("looking along the ground a cell runs %v down the screen, want less than the 2:1 view's %v", low, row)
	}
	if _, lift := testProjection.tilted(math.Pi/2).Project(0, 0, 10); !near(lift, 0) {
		t.Errorf("looking straight down a height lifts a point by %v, want nothing", lift)
	}
	if p := testProjection.tilted(0.01); !near(p.Pitch, defaultMinPitch) {
		t.Errorf("tilted to 0.01 the pitch is %v, want held at the default %v", p.Pitch, defaultMinPitch)
	}
	if p := lower.tilted(0.01); !near(p.Pitch, math.Pi/18) {
		t.Errorf("with a MinPitch of ten degrees, tilted to 0.01 the pitch is %v, want held at ten degrees", p.Pitch)
	}
}
