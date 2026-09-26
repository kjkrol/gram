package isometry

import "testing"

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
