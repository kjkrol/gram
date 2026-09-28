package world_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
)

// A kilometre a cell of 32 world units: the ground sinks under an eye's level as the Earth curves
// and the air bends the line of sight back, a man's eye sees level ground some 5 km off, and the
// clear air lets one see 40 km; without a scale the world is flat and its air clear.
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
	clear := s.Visibility(world.Weather{})
	if math.Abs(clear*s.Metres-world.ClearAir) > 1e-6 {
		t.Errorf("clear air lets one see %v m, want %v", clear*s.Metres, world.ClearAir)
	}
	if rain := s.Visibility(world.Weather{Rain: 1, Clouds: 1}); rain >= clear/5 {
		t.Errorf("in a downpour one sees %v world units, want well under a fifth of %v", rain, clear)
	}
	var flat world.Scale
	if flat.Bend() != 0 || !math.IsInf(flat.Horizon(2), 1) || flat.Visibility(world.Weather{}) != 0 || flat.Units(7) != 7 {
		t.Error("without a scale the world is not flat, its air not clear without end, or its units not its own")
	}
}

// The world works out how far one sees through the weather set, unless the weather says.
func TestSetWeather_SeesAsFarAsTheScaleSays(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 64, Height: 64}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}, Scale: world.Scale{Metres: 1000}})
	w.SetWeather(world.Weather{})
	if v := w.Weather().Visibility; math.Abs(v-40) > 1e-9 {
		t.Errorf("clear air on a world of kilometre units lets one see %v, want 40", v)
	}
	w.SetWeather(world.Weather{Visibility: 3})
	if v := w.Weather().Visibility; v != 3 || w.Weather().Frame().Visibility != 3 {
		t.Errorf("a weather seeing 3 far is seen %v far, want as it says", v)
	}
}
