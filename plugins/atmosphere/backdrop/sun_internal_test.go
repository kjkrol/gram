package backdrop

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
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

// sorting is a projection with depth, as a perspective has.
type sorting struct{ camera.Projection }

func (sorting) Sorts() bool { return true }
func (sorting) Wraps() bool { return false }

func TestBackdrop_TheSunStandsWhereItsWayVanishesThroughAPerspective(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	wcam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	sun, weather := sky.Sun{}, air.Weather{}
	b := New(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun }, func() air.Weather { return weather })
	cam := raying{height: 30}
	sun = sky.Sun{Dir: eyedF, Strength: 0.7}
	p := b.plan(cam)
	if math.Abs(float64(p.SunRadius-100*discAngle)) > 1e-3 || math.Abs(float64(p.SunAt[0]-50)) > 0.5 || math.Abs(float64(p.SunAt[1]-50)) > 0.5 || p.SunDisc[3] != 1 {
		t.Fatalf("the sun ahead stands at %v, %v wide in %v; want the middle of the screen, discAngle of the focal length, opaque", p.SunAt, p.SunRadius, p.SunDisc)
	}
	if z := b.plan(zoomed{cam}); math.Abs(float64(z.SunRadius-2*p.SunRadius)) > 1e-3 || math.Abs(float64(z.SunAt[0]-50)) > 0.5 {
		t.Errorf("zoomed in twice the sun is %v wide at %v, want twice %v in the middle", z.SunRadius, z.SunAt, p.SunRadius)
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
	if p := b.plan(shifted{Camera: wcam, dx: 150}); p.SunRadius != 0 {
		t.Errorf("the sun seen from above shows at %v: no way vanishes", p.SunAt)
	}
}

// A sun whose middle has sunk a little under the horizon still has its disc drawn, for the ground
// to hide what of it has set; well under, none. At night the stars come out and the moon, full
// and opposite the sun, has its face lit towards the eye.
func TestBackdrop_TheSunSetsAndTheNightComesOut(t *testing.T) {
	b := New(world.SpaceCfg{Width: 100, Height: 100}, world.Scale{}, nil, nil)
	cam := eyed{}
	if _, r, disc, _ := b.sunOn(cam, 100, 100, norm3([3]float32{0, -1, -0.02}), render.Light{1, 0.6, 0.3}, 0); r == 0 || disc[1] >= 1 {
		t.Errorf("a sun just under the horizon has a disc %v wide in %v, want one, reddened", r, disc)
	}
	if _, r, _, _ := b.sunOn(cam, 100, 100, norm3([3]float32{0, -1, -0.2}), render.Light{1, 1, 1}, 0); r != 0 {
		t.Errorf("a sun well under the horizon has a disc %v wide, want none", r)
	}
	var p skyPlan
	night := celestial.Heavens{Sun: norm3([3]float32{0, 1, -0.4}), Moon: eyedF, Full: 1, Pole: [3]float32{0, 0, 1}}
	b.nightOn(&p, cam, camera.RayField{Dir: eyedF, DDX: eyedR, DDY: [3]float32{-eyedU[0], -eyedU[1], -eyedU[2]}}, 100, 100, night, 0)
	if p.Stars < 0.99 || p.MoonRadius == 0 || p.MoonLight[2] < 0.9 {
		t.Errorf("at night stars %v, the moon %v wide lit from %v; want the stars out and the full moon lit towards the eye", p.Stars, p.MoonRadius, p.MoonLight)
	}
}

// What WithShown switches off does not show: without the stars the night is starless, without the
// moon it has no disc.
func TestBackdrop_ShowsTheStarsAndTheMoonOnlyAsShownSays(t *testing.T) {
	cam := eyed{}
	field := camera.RayField{Dir: eyedF, DDX: eyedR, DDY: [3]float32{-eyedU[0], -eyedU[1], -eyedU[2]}}
	night := celestial.Heavens{Sun: norm3([3]float32{0, 1, -0.4}), Moon: eyedF, Full: 1, Pole: [3]float32{0, 0, 1}}
	for _, c := range []struct{ stars, moon bool }{{true, false}, {false, true}, {false, false}} {
		b := New(world.SpaceCfg{Width: 100, Height: 100}, world.Scale{}, nil, nil).
			WithShown(func() (bool, bool) { return c.stars, c.moon })
		var p skyPlan
		b.nightOn(&p, cam, field, 100, 100, night, 0)
		if (p.Stars > 0) != c.stars || (p.MoonRadius > 0) != c.moon {
			t.Errorf("shown stars %v, moon %v: stars %v, the moon %v wide", c.stars, c.moon, p.Stars, p.MoonRadius)
		}
	}
}

// norm3 is a the length of 1.
func norm3(a [3]float32) [3]float32 {
	n := float32(math.Sqrt(float64(dot3(a, a))))
	return [3]float32{a[0] / n, a[1] / n, a[2] / n}
}

// zoomed is raying zoomed in twice: its focal length doubled, the middle of its screen kept.
type zoomed struct{ raying }

func (z zoomed) Rays() (camera.RayField, bool) {
	f, _ := z.raying.Rays()
	for k := range 3 {
		f.DDX[k] /= 2
		f.DDY[k] /= 2
		f.Dir[k] = eyedF[k] - f.DDX[k]*50 - f.DDY[k]*50
	}
	return f, true
}

func (z zoomed) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	x, y, ok := z.raying.Vanish(dx, dy, dz)
	return 50 + 2*(x-50), 50 + 2*(y-50), ok
}
