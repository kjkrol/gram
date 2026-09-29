package atmosphere

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// raying is eyed telling which way each screen point looks, from an eye height up.
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

// Rays is every line of sight of raying at once, as Ray has them: from the eye, the way of the
// screen point affine in it.
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

// Through a camera that says which way each screen point looks the sky runs from the horizon's
// colour up to a deeper one overhead, along the camera's lines of sight, and under a cloud layer
// the clouds are drawn on it; from above the layer, none.
func TestBackdrop_DrawsTheSkyFromTheHorizonUpAndTheCloudsOnIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	sun := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.7, Sky: render.Light{0.5, 0.7, 1}}
	weather := air.Weather{}
	b := NewBackdrop(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun }, func() air.Weather { return weather })
	p := b.plan(raying{height: 30})
	if p.None || p.Flat || p.Clouds {
		t.Fatalf("under a clear sky through a perspective: %+v, want the sky along the lines of sight and no clouds", p)
	}
	if o, h := p.Overhead, p.Horizon; !(o[0] < h[0] && o[1] < h[1]) || o[2] < h[2]-0.1 {
		t.Errorf("overhead %v, at the horizon %v: want the sky deeper overhead", o, h)
	}
	if f, _ := (raying{height: 30}).Rays(); p.Field != f || p.Eye != [3]float32{50, 150, 30} || p.View != [2]float32{100, 100} {
		t.Errorf("the sky looks along %+v from %v over %v, want the camera's lines of sight from its eye over its viewport", p.Field, p.Eye, p.View)
	}
	weather = air.Weather{Clouds: 1}
	if p := b.plan(raying{height: 30}); !p.Clouds || p.CloudHeight != air.Base(world.Scale{}) {
		t.Errorf("under full cloud from 30 up: clouds %v %v up, want them on the layer %v up", p.Clouds, p.CloudHeight, air.Base(world.Scale{}))
	}
	if p := b.plan(raying{height: air.Base(world.Scale{}) + 1}); p.Clouds {
		t.Error("from above the cloud layer the clouds are drawn on the sky")
	}
}
