package render

import (
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

func TestSprites_TurnIntoTheFogsColourAsFarAsTheirFog(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	blue := items(func(f *Frame) { f.Uniform("Fog", 0, 0, 1) })
	all := shot(t, groundOf(a), &figureSource{atlas: a, z: 10, id: 2, fog: 1}, blue)
	if got := all(24, 24); got != [4]byte{0, 0, 255, 255} {
		t.Errorf("all in the fog the figure shows %v, want the fog's blue", got)
	}
	half := shot(t, groundOf(a), &figureSource{atlas: a, z: 10, id: 2, fog: 0.5}, blue)
	if got := half(24, 24); got[0] < 100 || got[0] > 120 || got[2] < 120 || got[2] > 135 {
		t.Errorf("half in the fog the figure shows %v, want half its red, half the blue", got)
	}
	clear := shot(t, groundOf(a), &figureSource{atlas: a, z: 10, id: 2}, blue)
	if got := clear(24, 24); got != [4]byte{220, 0, 0, 255} {
		t.Errorf("out of the fog the figure shows %v, want it red", got)
	}
}

type plainAtlas struct{}

func (plainAtlas) Atlas() *Image                            { return nil }
func (plainAtlas) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (plainAtlas) White() (u, v float32)                    { return 0, 0 }

func TestFrame_FogTurnsARectangleIntoTheFogToo(t *testing.T) {
	var f Frame
	f.Reset(cameratest.NewFromSpace(64, 64, 0))
	f.SpriteRect(f.At(8, 8, 0), plainAtlas{}, 1, 0, 0, 16, 16, Even(1))
	f.Fog([4]float32{0.5, 0.5, 0.5, 0.5})
	for _, v := range f.verts[len(f.verts)-4:] {
		if v.ColorA != 1+fogSpan*0.5 {
			t.Fatalf("a rectangle's corner has alpha %v, want %v for half the fog", v.ColorA,
				1+fogSpan*0.5)
		}
	}
}
