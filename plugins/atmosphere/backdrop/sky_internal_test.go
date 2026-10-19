package backdrop

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type raying struct {
	eyed
	height float32
}

func (r raying) Eye() (float32, float32, float32, bool) { return 50, 150, r.height, true }

func (raying) Ray(sx, sy float32) (float32, float32, float32, bool) {
	d := [3]float32{}
	for k := range d {
		d[k] = eyedF[k] + eyedR[k]*(sx-50)/100 + eyedU[k]*(50-sy)/100
	}
	n := float32(math.Sqrt(float64(dot3(d, d))))
	return d[0] / n, d[1] / n, d[2] / n, true
}

func (r raying) Rays() (camera.RayField, bool) {
	f := camera.RayField{Origin: [3]float32{50, 150, r.height}}
	for k := range 3 {
		f.Dir[k] = eyedF[k] - eyedR[k]*0.5 + eyedU[k]*0.5
		f.DDX[k] = eyedR[k] / 100
		f.DDY[k] = -eyedU[k] / 100
	}
	return f, true
}

var _ camera.Rayer = raying{}
var _ camera.Rays = raying{}
var _ camera.Eyed = raying{}

func TestBackdrop_DrawsTheSkyFromTheHorizonUpAndTheCloudsOnIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	sun := light.Distant{Dir: [3]float32{0, 0, 1}, Strength: 0.7, AmbientColor: render.Light{0.5, 0.7, 1}}
	weather := air.Weather{}
	b := New(w.Res.Config.Space, func() light.Distant { return sun },
		func() air.Weather { return weather })
	p := b.plan(raying{height: 30})
	if p.None || p.Flat || p.Clouds {
		t.Fatalf("under a clear sky through a perspective: %+v, want the sky along the lines of sight and no clouds", p)
	}
	if f, _ := (raying{height: 30}).Rays(); p.Field != f || p.Eye != [3]float32{50, 150, 30} || p.View != [2]float32{100, 100} {
		t.Errorf("the sky looks along %+v from %v over %v, want the camera's lines of sight from its eye over its viewport", p.Field, p.Eye, p.View)
	}
	bases := air.Bases(world.Scale{})
	weather = air.Weather{Clouds: 1, Bases: bases}
	for _, z := range []float32{30, bases[air.Low] + 1} {
		if p := b.plan(raying{height: z}); !p.Clouds {
			t.Errorf("under full cloud from %v up the sky shows none", z)
		}
	}
	if p := b.plan(raying{height: bases[air.High] + 1}); p.Clouds {
		t.Error("from above both layers the clouds are drawn on the sky")
	}
}
