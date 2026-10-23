package render

import (
	"slices"
	"testing"

	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

func TestSprites_DrawsTheLowerFirstAndAtOneHeightTheLyingFirst(t *testing.T) {
	var s Sprites
	var f Frame // through no camera: a point's height its key
	add := func(mark, z float32, under bool) {
		s.Layer(f.At(0, 0, z), under)
		s.piece(nil)
		s.instances = append(s.instances, mark, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, z, 0, 0, 0)
	}
	add(1, 40, false) // a gate's top
	add(2, 32, false) // a walker 32 tall
	add(3, 0, true)   // a trapdoor under him
	add(4, 32, true)  // a deck as high as his head
	drawn, runs := s.byLayer()
	var got []float32
	for k := 0; k < len(drawn); k += spriteFloats {
		got = append(got, drawn[k])
	}
	if want := []float32{3, 4, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("drawn %v, want %v", got, want)
	}
	if len(runs) != 1 || runs[0].count != 4 {
		t.Errorf("runs %+v, want one of the four", runs)
	}
	if n := testing.AllocsPerRun(100, func() { s.byLayer() }); n != 0 {
		t.Errorf("%v allocations a frame, want none", n)
	}
}

func TestSprites_DrawTwiceDrawsTheSameSpritesUntilReset(t *testing.T) {
	gputest.Need(t)
	atlas := NewAtlas()
	atlas.Add(SpriteID(1), 8, Solid(white))
	atlas.Close()
	cam := cameratest.NewFromSpace(64, 64, 0)
	cam.SetViewport(64, 64)
	var s Sprites
	s.Rect(atlas, 1, 16, 16, 48, 48, 0, 0, 1, 1, Light{1, 1, 1})
	middle := func() [4]byte {
		screen := NewImage(64, 64)
		s.Draw(Target{Screen: screen}, cam, Uniforms{})
		return pixelsOf(screen)(32, 32)
	}
	for i := range 2 {
		if p := middle(); p != [4]byte{255, 255, 255, 255} {
			t.Errorf("drawn a %d. time the sprite shows %v, want it white", i+1, p)
		}
	}
	s.Reset()
	if p := middle(); p[3] != 0 {
		t.Errorf("after Reset the sprite still shows %v, want nothing drawn", p)
	}
}

func TestSprites_TurnIntoTheFogsColourAsFarAsTheirFog(t *testing.T) {
	gputest.Need(t)
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
