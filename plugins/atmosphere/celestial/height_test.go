package celestial_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
)

func TestHeights_ABodyRisesAndTheSunSetsIntoTheNightAtTheirEdges(t *testing.T) {
	for _, c := range []struct {
		h                  float32
		up                 bool
		risen, gone, night float32
	}{
		{h: 0.5, up: true, risen: 1, gone: 0, night: 0},
		{h: 0, up: true, risen: 0, gone: 0, night: 0},
		{h: -0.05, up: true, risen: 0, gone: 0, night: 0},
		{h: -0.2, up: false, risen: 0, gone: 0.741, night: 1},
		{h: -0.3, up: false, risen: 0, gone: 1, night: 1},
	} {
		up, risen := celestial.Up(c.h), celestial.Risen(c.h)
		gone, night := celestial.Gone(c.h), celestial.Night(c.h)
		if up != c.up || risen != c.risen || abs(gone-c.gone) > 1e-3 || night != c.night {
			t.Errorf("at %v: up %v, risen %v, gone %v, night %v; want %v, %v, %v, %v",
				c.h, up, risen, gone, night, c.up, c.risen, c.gone, c.night)
		}
	}
}

func abs(x float32) float32 { return max(x, -x) }
