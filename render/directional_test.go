package render

import (
	"testing"

	"github.com/kjkrol/gram/internal/f32"
)

func TestEndpointAt_DiagonalReachesTheCorner(t *testing.T) {
	const size = 10
	const tolerance = 1e-4

	cases := []struct {
		name         string
		angleDeg     float64
		wantX, wantY float32
	}{
		{"east (cardinal, right edge midpoint)", 0, 10, 5},
		{"north (cardinal, top edge midpoint)", 90, 5, 0},
		{"west (cardinal, left edge midpoint)", 180, 0, 5},
		{"south (cardinal, bottom edge midpoint)", 270, 5, 10},
		{"north-east corner", 45, 10, 0},
		{"north-west corner", 135, 0, 0},
		{"south-west corner", 225, 0, 10},
		{"south-east corner", 315, 10, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x, y := endpointAt(size, c.angleDeg)
			if f32.Abs(x-c.wantX) > tolerance || f32.Abs(y-c.wantY) > tolerance {
				t.Errorf("endpointAt(%d, %v) = (%v,%v), want (%v,%v)", size, c.angleDeg, x, y, c.wantX, c.wantY)
			}
		})
	}
}
