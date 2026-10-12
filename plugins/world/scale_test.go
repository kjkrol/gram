package world_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
)

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

func TestScale_FiguresBiggerThanLife(t *testing.T) {
	life := world.Scale{Metres: 10}
	if f := life.Figure(2); f != 0.2 {
		t.Errorf("a man 2 m tall is %v world units, want 0.2 as in life", f)
	}
	token := world.Scale{Metres: 10, Figures: 15}
	if f := token.Figure(2); math.Abs(f-3) > 1e-9 || token.Units(2) != 0.2 {
		t.Errorf("a man 2 m tall is %v world units and 2 m %v, want 3 and 0.2", f, token.Units(2))
	}
}
