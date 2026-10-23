package compose

import (
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

type plainAtlas struct{}

func (plainAtlas) Atlas() *Image                            { return nil }
func (plainAtlas) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (plainAtlas) White() (u, v float32)                    { return 0, 0 }

func TestFrame_FogTurnsARectangleIntoTheFogToo(t *testing.T) {
	var f Frame
	f.Reset(cameratest.NewFromSpace(64, 64, 0))
	f.SpriteRect(f.At(8, 8, 0), plainAtlas{}, 1, 0, 0, 16, 16, even(1))
	f.Fog([4]float32{0.5, 0.5, 0.5, 0.5})
	for _, v := range f.verts[len(f.verts)-4:] {
		if v.ColorA != 1+fogSpan*0.5 {
			t.Fatalf("a rectangle's corner has alpha %v, want %v for half the fog", v.ColorA,
				1+fogSpan*0.5)
		}
	}
}
