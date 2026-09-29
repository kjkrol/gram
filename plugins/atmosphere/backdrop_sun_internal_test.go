package atmosphere

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
)

// eyed is a 100 x 100 screen through a perspective: an eye at (50, 150, 30) looking along -y and
// a little up through a focal length of 100, off the world, so the sky shows.
type eyed struct{ camera.Camera }

// The eye's frame: forward, right and up.
var (
	eyedF = [3]float32{0, -0.9487, 0.3162}
	eyedR = [3]float32{1, 0, 0}
	eyedU = [3]float32{0, 0.3162, 0.9487}
)

func (eyed) Viewport() (float32, float32)                   { return 100, 100 }
func (eyed) Unproject(sx, sy, _ float32) (float32, float32) { return sx + 150, sy }
func (eyed) Projection() camera.Projection                  { return sorting{} }
func (eyed) Project(x, y, z float32) (float32, float32) {
	d := [3]float32{x - 50, y - 150, z - 30}
	ahead := max(dot3(d, eyedF), 1)
	return 50 + 100*dot3(d, eyedR)/ahead, 50 - 100*dot3(d, eyedU)/ahead
}

func (eyed) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	d := [3]float32{dx, dy, dz}
	ahead := dot3(d, eyedF)
	if ahead <= 0 {
		return 0, 0, false
	}
	return 50 + 100*dot3(d, eyedR)/ahead, 50 - 100*dot3(d, eyedU)/ahead, true
}

func dot3(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// sorting is a projection with depth, as a perspective has.
type sorting struct{ camera.Projection }

func (sorting) Sorts() bool { return true }
func (sorting) Wraps() bool { return false }

func TestBackdrop_TheSunStandsWhereItsWayVanishesThroughAPerspective(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	sun, weather := sky.Sun{}, air.Weather{}
	b := NewBackdrop(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun }, func() air.Weather { return weather })
	cam := raying{height: 30}
	sun = sky.Sun{Dir: eyedF, Strength: 0.7}
	p := b.plan(cam)
	if p.SunRadius != 100*sunRadius || math.Abs(float64(p.SunAt[0]-50)) > 0.5 || math.Abs(float64(p.SunAt[1]-50)) > 0.5 || p.SunDisc[3] != 1 {
		t.Fatalf("the sun ahead stands at %v, %v wide in %v; want the middle of the screen, a fortieth of it, opaque", p.SunAt, p.SunRadius, p.SunDisc)
	}
	sun = sky.Sun{Dir: [3]float32{0.2, -0.8855, 0.5059}, Strength: 0.7} // a fifth up and to the right of the way looked
	if p := b.plan(cam); p.SunRadius == 0 || p.SunAt[0] <= 50 || p.SunAt[1] >= 50 {
		t.Errorf("the sun up to the right stands at %v, want right of and above the middle", p.SunAt)
	}
	sun = sky.Sun{Dir: [3]float32{0, 0.9487, 0.3162}, Strength: 0.7} // behind the eye
	if p := b.plan(cam); p.SunRadius != 0 {
		t.Errorf("the sun behind the eye shows at %v", p.SunAt)
	}
	sun = sky.Sun{Dir: [3]float32{0, -0.7, -0.7}, Strength: 0.7} // set
	if p := b.plan(cam); p.SunRadius != 0 {
		t.Errorf("the sun under the horizon shows at %v", p.SunAt)
	}
	sun = sky.Sun{Dir: eyedF, Strength: 0.7}
	weather = air.Weather{Clouds: 1}
	if p := b.plan(cam); p.SunRadius != 0 {
		t.Errorf("the sun under full cloud shows at %v", p.SunAt)
	}
	weather = air.Weather{}
	if p := b.plan(shifted{Camera: w.Camera(), dx: 150}); p.SunRadius != 0 {
		t.Errorf("the sun seen from above shows at %v: no way vanishes", p.SunAt)
	}
}
