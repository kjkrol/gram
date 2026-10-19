package render

import (
	"math"
	"testing"
)

func TestGeoM_RotateTurnsWhatCameBefore(t *testing.T) {
	var g GeoM
	g.Scale(2, 1)
	g.Translate(1, 0)
	g.Rotate(math.Pi / 2)
	// (1, 0) scaled to (2, 0), moved to (3, 0), turned a quarter from x towards y: (0, 3)
	if x, y := g.Apply(1, 0); math.Abs(x) > 1e-9 || math.Abs(y-3) > 1e-9 {
		t.Errorf("(1, 0) went to (%v, %v), want (0, 3)", x, y)
	}
}
