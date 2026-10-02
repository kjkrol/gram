package terrain

import (
	"image"
	"testing"

	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
)

// As the sun turns the shade is baked anew a strip a frame, and a round of strips holds what a
// bake of the whole at once holds; the sun leapt, it is baked whole at once. Coarse, it is baked
// half as fine a side.
func TestRenderer_BakesTheShadeInStripsAsTheSunTurns(t *testing.T) {
	needGPU(t)
	cam := icamera.NewFromSpace(512, 512, 0)
	sun := func(x float32) render.Uniforms { // low in the east or in the west: the ridge's shadow to either side
		return render.UniformsOf(map[string]any{"Sun": []float32{x, 0.1, 0.3}, "SunStrength": []float32{1}})
	}
	nudged := func(x float32) render.Uniforms { // half a degree on: turned, not leapt
		return render.UniformsOf(map[string]any{"Sun": []float32{x, 0.1 + 0.0055, 0.3}, "SunStrength": []float32{1}})
	}
	shade := func(r *Renderer) []byte {
		cols, rows, _, _, _ := r.ground.Lattice()
		ks := bakedScale(cols, rows, r.coarse)
		part := r.baked.SubImage(image.Rect(0, 0, ks*(cols-1), ks*(rows-1)))
		pix := make([]byte, 4*part.Bounds().Dx()*part.Bounds().Dy())
		part.ReadPixels(pix)
		return pix
	}
	frame := func(r *Renderer, u render.Uniforms) {
		if !r.prepare(cam, u) {
			t.Fatal("nothing to draw")
		}
		r.bake(u)
	}
	east := New(&hill{}, none{}, stillSky{}, Config{Shadows: true})
	frame(east, sun(0.9))
	west := New(&hill{}, none{}, stillSky{}, Config{Shadows: true})
	frame(west, nudged(0.9))
	want, before := shade(west), shade(east)

	r := New(&hill{}, none{}, stillSky{}, Config{Shadows: true})
	frame(r, sun(0.9))
	frame(r, nudged(0.9)) // the first strip of a round
	got := shade(r)
	w := len(got) / (4 * 128)        // pixels a row: 16 cells of 8 down
	per := 128 / shadeStrips * w * 4 // bytes a strip
	if string(got[:per]) != string(want[:per]) || string(got[per:]) != string(before[per:]) {
		t.Fatal("after one frame of a turned sun the shade is not its first strip anew and the rest as it was")
	}
	for range shadeStrips - 1 {
		frame(r, nudged(0.9))
	}
	if got := shade(r); string(got) != string(want) {
		t.Error("after a round of strips the shade is not what a bake of the whole holds")
	}
	if string(want) == string(before) {
		t.Fatal("the sun turned casts the same shade: the test sees nothing")
	}
	frame(r, sun(-0.9)) // leapt to the other side: all of it at once
	west = New(&hill{}, none{}, stillSky{}, Config{Shadows: true})
	frame(west, sun(-0.9))
	if string(shade(r)) != string(shade(west)) {
		t.Error("the sun leapt, the shade is not baked whole at once")
	}

	r.Coarse(true)
	frame(r, sun(-0.9))
	if cols, rows, _, _, _ := r.ground.Lattice(); bakedScale(cols, rows, true) != 4 || len(shade(r)) != 4*(4*(cols-1))*(4*(rows-1)) {
		t.Errorf("coarse, the baked image is %v, want 4 pixels a cell", r.baked.Bounds())
	}
}
