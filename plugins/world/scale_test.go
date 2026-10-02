package world_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
)

// A kilometre a cell of 32 world units: the ground sinks under an eye's level as the Earth curves
// and the air bends the line of sight back, and a man's eye sees level ground some 5 km off;
// without a scale the world is flat.
func TestScale_TheEarthUnderTheWorld(t *testing.T) {
	s := world.Scale{Metres: 1000.0 / 32}
	if u := s.Units(2); math.Abs(u-0.064) > 1e-9 {
		t.Errorf("2 m is %v world units, want 0.064", u)
	}
	drop := s.Drop(s.Units(60000)) * s.Metres // 60 km off, in metres
	if want := 60000.0 * 60000 * (1 - world.Refraction) / (2 * world.EarthRadius); math.Abs(drop-want) > 1e-6*want || drop < 240 || drop > 250 {
		t.Errorf("60 km off the ground sinks %v m, want about 246", drop)
	}
	if h := s.Horizon(s.Units(2)) * s.Metres; h < 5300 || h > 5500 {
		t.Errorf("an eye 2 m up sees level ground %v m off, want about 5.4 km", h)
	}
	var flat world.Scale
	if flat.Bend() != 0 || !math.IsInf(flat.Horizon(2), 1) || flat.Units(7) != 7 {
		t.Error("without a scale the world is not flat, or its units not its own")
	}
}
