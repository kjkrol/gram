package backdrop

import (
	"testing"

	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
)

func TestBackdrop_TheSunHidesBehindAFrontBeforeItReachesTheEye(t *testing.T) {
	sun := sky.Sun{Dir: eyedF, Strength: 0.7}
	for _, c := range []struct {
		name  string
		side  float32 // along y: the clouds come from y's low side, the sun's, or its high
		shown bool
	}{
		{"a front coming from the sun's side", -1, false},
		{"a front coming from behind", 1, true},
	} {
		d := air.NewDome(200, 200, 30000)
		d.Own()
		d.Carry([2]float32{}, func(_, y float32) (air.Mass, int16) {
			if (y-100)*c.side > 5000 {
				return air.Mass{Cover: 1}, 0
			}
			return air.Mass{}, 0
		}, true)
		weather := air.Weather{Dome: d, Bases: air.Bases(world.Scale{})}
		b := New(world.SpaceCfg{Width: 200, Height: 200},
			func() sky.Sun { return sun }, func() air.Weather { return weather })
		p := b.plan(raying{height: 30})
		if (p.SunRadius > 0) != c.shown {
			t.Errorf("%s: the sun's disc %v wide, want shown %v", c.name, p.SunRadius, c.shown)
		}
		if cover := weather.At(50, 150).Cover; cover != 0 {
			t.Errorf("%s: over the eye the cover is %v, want the front not there yet", c.name,
				cover)
		}
	}
}
