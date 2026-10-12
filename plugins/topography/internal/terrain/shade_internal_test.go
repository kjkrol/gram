package terrain

import (
	"image"
	"testing"

	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
)

func TestRenderer_BakesTheShadeInStripsAsTheSunTurns(t *testing.T) {
	needGPU(t)
	cam := icamera.NewFromSpace(512, 512, 0)
	sun := func(x float32) render.Uniforms { // low in the east or the west
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
	frame(r, nudged(0.9))
	got := shade(r)
	w := len(got) / (4 * 128)        // pixels a row
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
	frame(r, sun(-0.9)) // leapt to the other side
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

type level struct{}

func (level) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return 17, 17, 32, make([]float32, 17*17), true
}

func (level) Version() uint64 { return 0 }

func TestRenderer_BakesTheShadowsOfTheBlocks(t *testing.T) {
	needGPU(t)
	cam := icamera.NewFromSpace(512, 512, 0)
	east := render.UniformsOf(map[string]any{
		"Sun":         []float32{0.7071, 0, 0.7071},
		"SunStrength": []float32{1},
	})
	r := New(level{}, none{}, stillSky{}, Config{Shadows: true})
	// a deck 64 along x either side of (256, y), 16 across, from 40 up to 50, in two pieces
	deckAt := float32(256)
	r.findBlocks = func(dst []block) []block {
		piece := block{ux: 1, along: 32, wide: 16, bottom: 40, top: 50, y: deckAt}
		a, b := piece, piece
		a.x, b.x = 224, 288
		return append(dst, a, b)
	}
	frame := func() {
		if !r.prepare(cam, east) {
			t.Fatal("nothing to draw")
		}
		r.bake(east)
	}
	shadeAt := func(x, y float32) byte {
		pix := make([]byte, 4)
		r.baked.SubImage(image.Rect(int(x/4), int(y/4), int(x/4)+1, int(y/4)+1)).ReadPixels(pix)
		return pix[0]
	}
	frame()
	if n := len(r.blocks) / 8; n != 1 {
		t.Errorf("the deck's two pieces are baked as %d blocks, want one", n)
	}
	// the line to the sun rises 1 east for 1 up: from x it meets the deck over x+40 to x+50
	if v := shadeAt(200, 256); v > 10 {
		t.Errorf("the ground the deck hides from the sun is lit %d, want dark", v)
	}
	if v := shadeAt(310, 256); v < 245 {
		t.Errorf("the ground right under the deck's sunny end is lit %d, want lit: the shadow "+
			"falls away from the sun", v)
	}
	if v := shadeAt(200, 210); v < 245 {
		t.Errorf("the ground beside the shadow is lit %d, want lit", v)
	}
	if v := shadeAt(200, 274); v < 25 || v > 230 {
		t.Errorf("the ground past the shadow's edge is lit %d, want half in the penumbra", v)
	}
	deckAt = 320
	frame()
	if v := shadeAt(200, 256); v < 245 {
		t.Errorf("the deck moved away, the ground is lit %d, want baked anew lit", v)
	}
	if v := shadeAt(200, 320); v > 10 {
		t.Errorf("the deck moved over, the ground is lit %d, want baked anew dark", v)
	}
}
