package landscape_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/landscape"
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
	landscape.Overcast(&f, render.Box(0, 0, 8, 8))
	if f.Len() != 1 {
		t.Fatalf("under a clear sky %d pieces, want the sprite alone", f.Len())
	}
	f.Weather(render.Weather{Clouds: 0.5})
	landscape.Overcast(&f, render.Box(0, 0, 8, 8))
	if f.Len() != 2 {
		t.Fatalf("under clouds %d pieces, want the sprite and the clouds' shadow over it", f.Len())
	}
}
