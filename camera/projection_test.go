package camera_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
)

var iso = camera.Isometric{Cell: 32, TileW: 64, TileH: 32, HeightUnit: 2}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestIsometric_ACellIsADiamondAndHeightLiftsAPoint(t *testing.T) {
	if sx, sy := iso.Project(32, 0, 0); !near(sx, 32) || !near(sy, 16) {
		t.Errorf("one cell along x projects to (%v, %v), want (32, 16): down-right", sx, sy)
	}
	if sx, sy := iso.Project(0, 32, 0); !near(sx, -32) || !near(sy, 16) {
		t.Errorf("one cell along y projects to (%v, %v), want (-32, 16): down-left", sx, sy)
	}
	if _, sy := iso.Project(0, 0, 10); !near(sy, -20) {
		t.Errorf("a point 10 up projects to y %v, want -20: two units a height unit", sy)
	}
}

func TestIsometric_UnprojectInvertsProject(t *testing.T) {
	for _, p := range [][3]float32{{0, 0, 0}, {100, 40, 0}, {13.5, 250, 7}, {-20, 5, 30}} {
		sx, sy := iso.Project(p[0], p[1], p[2])
		x, y := iso.Unproject(sx, sy, p[2])
		if !near(x, p[0]) || !near(y, p[1]) {
			t.Errorf("point %v went to (%v, %v) and came back as (%v, %v)", p, sx, sy, x, y)
		}
	}
}

func TestIsometric_DepthIsTheRowOfTheCellUnderThePoint(t *testing.T) {
	back, front := iso.Depth(0, 0, 0), iso.Depth(32, 32, 0)
	if back >= front {
		t.Errorf("depth at the origin %v is not behind (32, 32) %v", back, front)
	}
	if iso.Depth(32, 0, 0) != iso.Depth(0, 32, 0) {
		t.Error("two cells on the same row differ in depth")
	}
	if tile, standing := iso.Depth(48, 48, 0), iso.Depth(40, 60, 30); standing != tile || standing >= iso.Depth(64, 48, 0) {
		t.Errorf("a thing anywhere in a cell has depth %v, want its tile's %v, before the next row", standing, tile)
	}
}
