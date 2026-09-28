package render_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/internal/camera"

	"github.com/kjkrol/gram/render"
)

// the white texel of these tests' sheet
type sheet struct{}

func (sheet) Atlas() *ebiten.Image                        { return nil }
func (sheet) UV(render.SpriteID) (x0, y0, x1, y1 float32) { return 0, 0, 8, 8 }
func (sheet) White() (u, v float32)                       { return 40, 40 }

func TestOvercast_LaysTheCloudsShadowsOnlyUnderClouds(t *testing.T) {
	var f render.Frame
	f.Reset(camera.NewFromSpace(64, 64, 0))
	f.Sprite(render.Ground, 0, sheet{}, 0, render.Corners{{0, 0}, {8, 0}, {0, 8}, {8, 8}}, render.Even(1))
	thick := [4]float32{1, 1, 1, 1}
	f.Overcast(render.Box(0, 0, 8, 8), thick)
	if f.Len() != 1 {
		t.Fatalf("under a clear sky %d pieces, want the sprite alone", f.Len())
	}
	f.Weather(render.Weather{Clouds: 0.5})
	f.Overcast(render.Box(0, 0, 8, 8), [4]float32{}) // the clouds miss the piece: no shadow to lay
	if f.Len() != 1 {
		t.Fatalf("under clouds that miss the piece %d pieces, want the sprite alone", f.Len())
	}
	f.Overcast(render.Box(0, 0, 8, 8), thick)
	if f.Len() != 2 {
		t.Fatalf("under clouds %d pieces, want the sprite and the clouds' shadow over it", f.Len())
	}
	var frac float32
	f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
		frac = v[0].ColorA - 2 - 2*float32(render.CloudShadow())
	})
	if frac != 1 {
		t.Errorf("the shadow's corner carries clouds of %v, want the 1 given", frac)
	}
}

// The clouds' noise is smooth, 0 to 1, and drifts with the wind: a point under a cloud is under the
// same cloud once both have moved on together.
func TestCloudAt_IsSmoothNoiseCarriedByTheDrift(t *testing.T) {
	var lo, hi float32 = 1, 0
	for i := range 64 {
		for j := range 64 {
			x, y := float32(i)*50, float32(j)*50
			n := render.CloudAt(x, y, [2]float32{})
			lo, hi = min(lo, n), max(hi, n)
			if d := n - render.CloudAt(x+1, y+1, [2]float32{}); d > 0.02 || d < -0.02 {
				t.Errorf("the clouds' noise jumps by %v over a unit at (%v, %v)", d, x, y)
			}
			if d := n - render.CloudAt(x+300, y-120, [2]float32{300, -120}); d > 1e-4 || d < -1e-4 {
				t.Errorf("carried 300, -120 the clouds at (%v, %v) changed by %v", x, y, d)
			}
		}
	}
	if lo < 0 || hi > 1 || hi-lo < 0.3 {
		t.Errorf("the clouds' noise runs %v to %v, want within 0 to 1 and varied", lo, hi)
	}
	if render.CloudCover(0, 0.5) != 0 || render.CloudCover(1, 0.5) != 1 || render.CloudCover(1, 0) != 0 {
		t.Error("CloudCover: no cloud where the noise is low, all of it where high, none under a clear sky")
	}
	if render.Shadowed([4]float32{0.2, 0.2, 0.2, 0.2}, 0.5) || !render.Shadowed([4]float32{0.2, 0.2, 0.2, 0.9}, 0.5) {
		t.Error("Shadowed: a piece whose corners are clear is clear, one with a cloud over a corner is not")
	}
}
