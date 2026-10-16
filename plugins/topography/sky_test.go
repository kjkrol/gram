package topography_test

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/render"
)

type ownSky struct{ sun sky.Sun }

func (s ownSky) Sun() sky.Sun     { return s.sun }
func (s ownSky) Air() air.Weather { return air.Weather{} }

// a sky some other source puts on the frame
type framing struct{ sun sky.Sun }

func (framing) Init(*goke.SysInit) {}
func (s framing) Compose(f *render.Frame, _ camera.Camera) {
	s.sun.Frame(f)
	air.Weather{}.Frame(f, s.sun, 0, 0)
}

func TestPlugin_WithoutAnAtmosphereTheReliefHandsTheFrameItsStillSky(t *testing.T) {
	framed := func(sources ...render.Source) map[string][]float32 {
		c := render.NewComposer(sources...)
		c.DrawWorld(nil, icamera.NewFromSpace(256, 256, 0))
		return c.Uniforms()
	}
	relief := func() *topography.Plugin {
		w := topotest.NewWorld(0)
		b, _ := topotest.LevelBoard(w)
		return topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1})
	}
	got := framed(topotest.Drawn(t, relief())...)
	if s, fog := got["SunStrength"], got["Fog"]; len(s) != 1 || s[0] != sky.DefaultSun.Strength ||
		fog[0] != 1 || fog[2] != 1 {
		t.Errorf("under no atmosphere the frame has the sun %v and the horizon %v, want the "+
			"still sky's and white", s, fog)
	}
	lit := relief()
	lit.WithAtmosphere(ownSky{sun: sky.Sun{Strength: 0.3}})
	if s := framed(topotest.Drawn(t, lit)...)["SunStrength"]; len(s) == 1 && s[0] != 0 {
		t.Errorf("under an atmosphere the relief hands the frame the sun %v, want none: the "+
			"atmosphere's sky does", s)
	}
	other := framing{sun: sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.3}}
	for _, first := range []bool{true, false} {
		sources := append([]render.Source{other}, topotest.Drawn(t, relief())...)
		if !first {
			sources = append(topotest.Drawn(t, relief()), other)
		}
		if s := framed(sources...)["SunStrength"]; s[0] != 0.3 {
			t.Errorf("another sky composed first %v: the frame's sun %v, want the other's 0.3",
				first, s)
		}
	}
}
