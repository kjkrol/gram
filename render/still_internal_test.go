package render

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/camera"
)

// A still composed in world units is drawn where the camera it is drawn through shows the world:
// scaled by the zoom from the world point at the screen's top-left, its colours in the light
// handed to it, nothing drawn beyond it.
func TestStill_IsDrawnWhereTheCameraShowsTheWorld(t *testing.T) {
	needGPU(t)
	atlas := NewAtlas()
	atlas.Add(SpriteID(1), 4, Solid(color.RGBA{R: 200, G: 100, B: 40, A: 255}))
	atlas.Close()
	s := NewStill()
	s.Compose(100, 100, func(f *Frame, _ camera.Camera) {
		f.SpriteRect(Ground, 0, atlas, 1, 10, 10, 20, 20, Even(1))
	})
	if s.Len() != 1 {
		t.Fatalf("the still holds %d pieces, want 1", s.Len())
	}
	screen := NewImage(64, 64)
	pix := make([]byte, 4*64*64)
	s.Draw(screen, UniformsOf(map[string]any{}), 2, 5, 5, Light{0.5, 0.5, 0.5})
	screen.ReadPixels(pix)
	at := func(x, y int) [4]byte { i := 4 * (y*64 + x); return [4]byte(pix[i : i+4]) }
	// the world (10, 10) to (20, 20) seen from (5, 5) at 2 pixels a unit: the screen's 10 to 30
	if c := at(20, 20); c[0] < 98 || c[0] > 102 || c[1] < 48 || c[1] > 52 || c[3] != 255 {
		t.Errorf("inside the piece the screen is %v, want its colour in half light, (100, 50, 20)", c)
	}
	for _, p := range [][2]int{{8, 20}, {32, 20}, {20, 8}, {20, 32}} {
		if c := at(p[0], p[1]); c[3] != 0 {
			t.Errorf("outside the piece at %v the screen is %v, want nothing", p, c)
		}
	}
}
