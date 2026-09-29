package atmosphere

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
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

// Through a camera that says which way each screen point looks the sky is a mesh from the horizon
// up, deeper overhead, and under a cloud layer the clouds are drawn on the pieces looking up at
// it; from above the layer, none.
func TestBackdrop_DrawsTheSkyFromTheHorizonUpAndTheCloudsOnIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	sun := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.7, Sky: render.Light{0.5, 0.7, 1}}
	weather := air.Weather{}
	b := NewBackdrop(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun }, func() air.Weather { return weather })
	compose := func(cam camera.Camera) (quads, clouds int, top, bottom ebiten.Vertex) {
		var f render.Frame
		f.Reset(cam)
		b.Compose(&f, cam)
		f.Each(func(tier render.Tier, _ float32, verts []ebiten.Vertex) {
			if tier != render.Backdrop || len(verts) != 4 {
				return
			}
			if verts[0].ColorA > 1.5 {
				clouds++
				return
			}
			quads++
			if verts[0].DstX == 0 && verts[0].DstY == 0 {
				top = verts[0]
			}
			if verts[2].DstX == 0 && verts[2].DstY == 100 {
				bottom = verts[2]
			}
		})
		return
	}
	quads, clouds, top, bottom := compose(raying{height: 30})
	if quads != 4 || clouds != 0 {
		t.Fatalf("under a clear sky: %d quads and %d cloud pieces, want the mesh's 4 and none", quads, clouds)
	}
	if !(top.ColorR < bottom.ColorR && top.ColorG < bottom.ColorG) || top.ColorB < bottom.ColorB-0.1 {
		t.Errorf("the top of the screen is %v %v %v and the bottom %v %v %v, want the sky deeper overhead than at the horizon", top.ColorR, top.ColorG, top.ColorB, bottom.ColorR, bottom.ColorG, bottom.ColorB)
	}
	weather = air.Weather{Clouds: 1}
	if quads, clouds, _, _ := compose(raying{height: 30}); quads != 4 || clouds < 2 || clouds > 4 {
		t.Errorf("under full cloud from 30 up: %d quads and %d cloud pieces, want 4 and the pieces with a corner looking up at the layer, 2 to 4", quads, clouds)
	}
	if quads, clouds, _, _ := compose(raying{height: air.Base(world.Scale{}) + 1}); quads != 4 || clouds != 0 {
		t.Errorf("from above the cloud layer: %d quads and %d cloud pieces, want 4 and none", quads, clouds)
	}
}
