package backdrop

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// shifted is a 100x100 screen over the world from x = dx on, drawn from above without wrapping.
type shifted struct {
	camera.Camera
	dx float32
}

func (s shifted) Viewport() (float32, float32)                   { return 100, 100 }
func (s shifted) Unproject(sx, sy, _ float32) (float32, float32) { return sx + s.dx, sy }

func TestBackdrop_FillsTheScreenWithTheSkyOnlyWhereTheGroundDoesNotCoverIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	sun := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.5, Sky: render.Light{0.5, 0.7, 1}}
	b := New(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun }, func() air.Weather { return air.Weather{} })
	if b.Tier() != render.Backdrop {
		t.Errorf("the sky comes on tier %v, want the Backdrop, before everything", b.Tier())
	}
	if p := b.plan(shifted{Camera: cam, dx: 50}); !p.None {
		t.Errorf("over the middle of the world the backdrop draws %+v, want nothing: the ground covers the screen", p)
	}
	p := b.plan(shifted{Camera: cam, dx: 150})
	if p.None || !p.Flat || math.Abs(float64(p.Horizon[0]-0.5)) > 0.01 || math.Abs(float64(p.Horizon[2]-1)) > 0.01 {
		t.Errorf("over the world's edge the backdrop draws %+v, want the whole screen flat in the sky's colour", p)
	}
}
