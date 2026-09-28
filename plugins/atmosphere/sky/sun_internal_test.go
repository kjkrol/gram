package sky

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
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

func dot3(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// sorting is a projection with depth, as a perspective has.
type sorting struct{ camera.Projection }

func (sorting) Sorts() bool { return true }
func (sorting) Wraps() bool { return false }

func TestBackdrop_TheSunStandsWhereItsWayVanishesThroughAPerspective(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	b := NewBackdrop(w)
	pieces := func(cam camera.Camera) (n int, fans [][2]float32, depth float32) {
		var f render.Frame
		f.Reset(cam)
		b.Compose(&f, cam)
		f.Each(func(tier render.Tier, d float32, verts []ebiten.Vertex) {
			n++
			if tier != render.Backdrop {
				t.Errorf("a piece on tier %v, want every one on the Backdrop", tier)
			}
			if len(verts) > 4 {
				fans, depth = append(fans, [2]float32{verts[0].DstX, verts[0].DstY}), d
			}
		})
		return
	}
	w.SetSun(world.Sun{Dir: eyedF, Strength: 0.7})
	n, fans, depth := pieces(eyed{})
	if n != 3 || len(fans) != 2 || math.IsInf(float64(depth), -1) {
		t.Fatalf("the sun ahead: %d pieces, %d fans at depth %v; want the sky and the sun's glow and disc after it", n, len(fans), depth)
	}
	for _, p := range fans {
		if math.Abs(float64(p[0]-50)) > 0.5 || math.Abs(float64(p[1]-50)) > 0.5 {
			t.Errorf("the sun ahead is drawn round (%v, %v), want the middle of the screen", p[0], p[1])
		}
	}
	w.SetSun(world.Sun{Dir: [3]float32{0.2, -0.8855, 0.5059}, Strength: 0.7}) // a fifth up and to the right of the way looked
	if _, fans, _ := pieces(eyed{}); len(fans) != 2 || fans[0][0] <= 50 || fans[0][1] >= 50 {
		t.Errorf("the sun up to the right is drawn at %v, want right of and above the middle", fans)
	}
	w.SetSun(world.Sun{Dir: [3]float32{0, 0.9487, 0.3162}, Strength: 0.7}) // behind the eye
	if n, fans, _ := pieces(eyed{}); n != 1 || len(fans) != 0 {
		t.Errorf("the sun behind the eye: %d pieces, %d fans; want the sky alone", n, len(fans))
	}
	w.SetSun(world.Sun{Dir: [3]float32{0, -0.7, -0.7}, Strength: 0.7}) // set
	if n, _, _ := pieces(eyed{}); n != 1 {
		t.Errorf("the sun under the horizon: %d pieces, want the sky alone", n)
	}
	w.SetSun(world.Sun{Dir: eyedF, Strength: 0.7})
	w.SetWeather(world.Weather{Clouds: 1})
	if n, _, _ := pieces(eyed{}); n != 1 {
		t.Errorf("the sun under full cloud: %d pieces, want the sky alone", n)
	}
	w.SetWeather(world.Weather{})
	if n, _, _ := pieces(shifted{Camera: w.Camera(), dx: 150}); n != 1 {
		t.Errorf("the sun seen from above: %d pieces, want the sky alone: no way vanishes", n)
	}
}
