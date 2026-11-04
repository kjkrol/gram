package instanced

import (
	"image/color"
	"slices"
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/gram/render/internal/atlas"
	"github.com/kjkrol/gram/render/internal/compose"
	"github.com/kjkrol/gram/render/internal/gfx"
)

func TestSprites_DrawsTheLowerFirstAndAtOneHeightTheLyingFirst(t *testing.T) {
	var s Sprites
	var f compose.Frame // through no camera: a point's height its key
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
	sheet := atlas.NewAtlas()
	sheet.Add(SpriteID(1), 8, solid(white))
	sheet.Close()
	cam := cameratest.NewFromSpace(64, 64, 0)
	cam.SetViewport(64, 64)
	var s Sprites
	s.Rect(sheet, 1, 16, 16, 48, 48, 0, 0, 1, 1, Light{1, 1, 1})
	middle := func() [4]byte {
		screen := gfx.NewImage(64, 64)
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

func TestSprites_PointHoldsItsBoxItsSpriteAndHowItIsDrawn(t *testing.T) {
	sheet := atlas.NewAtlas()
	sheet.Add(SpriteID(1), 8, solid(white))
	sheet.Add(SpriteID(2), 8, solid(white))
	sheet.Close()
	var s Sprites
	s.Fog(0.5)
	s.Take(compose.TakesRelief)
	s.Direct(0.75)
	s.Point(sheet, 1, 10, 20, 0.5, 3, 4, 6, Light{0.1, 0.2, 0.3})
	s.Point(sheet, 2, 30, 40, 1, 1, 0, 2, Light{1, 1, 1})
	u0, v0, u1, v1 := inset(sheet.UV(2))
	want := []float32{30, 40, 1, 1, u0, v0, u1, v1, 1, 1, 1, 0, 0.5,
		float32(compose.TakesRelief), 0.75, 2}
	if got := s.points[spriteFloats:]; !slices.Equal(got, want) {
		t.Errorf("the second point is %v, want %v", got, want)
	}
	if len(s.pointRuns) != 1 || s.pointRuns[0].count != 2 || len(s.runs) != 0 {
		t.Errorf("point runs %+v, runs %+v; want one of the two points and no piece",
			s.pointRuns, s.runs)
	}
	s.Reset()
	if len(s.points) != 0 || len(s.pointRuns) != 0 {
		t.Errorf("after Reset %d floats in %d runs, want none", len(s.points), len(s.pointRuns))
	}
	s.Point(sheet, 1, 0, 0, 1, 1, 0, 0, Light{})
	if n := testing.AllocsPerRun(100, func() {
		s.Reset()
		s.Point(sheet, 1, 0, 0, 1, 1, 0, 0, Light{})
	}); n != 0 {
		t.Errorf("%v allocations a point, want none", n)
	}
}

// drawPoint draws a point of a white sprite with a red corner over a box w by h at (x, y), fogged
// by fog into the blue, onto a 16 by 16 screen a pixel a unit and reads it
func drawPoint(t *testing.T, x, y, w, h, fog float32) func(x, y int) [4]byte {
	t.Helper()
	gputest.Need(t)
	sheet := atlas.NewAtlas()
	sheet.Add(SpriteID(1), 8, func(dst *atlas.Canvas, size int) {
		dst.FillRect(0, 0, float32(size), float32(size), white)
		dst.FillRect(0, 0, 2, 2, color.RGBA{R: 255, A: 255})
	})
	sheet.Close()
	cam := cameratest.NewFromSpace(16, 16, 0)
	cam.SetViewport(16, 16)
	var s Sprites
	s.Fog(fog)
	s.Point(sheet, 1, x, y, w, h, 0, 0, Light{1, 1, 1})
	screen := gfx.NewImage(16, 16)
	s.Draw(Target{Screen: screen}, cam, compose.UniformsOf(map[string]any{
		"Fog": []float32{0, 0, 1},
	}))
	return pixelsOf(screen)
}

func TestSprites_APointIsItsBoxAPixelAtLeastAsOpaqueAsItCovers(t *testing.T) {
	for _, tc := range []struct {
		name       string
		x, y, w, h float32
		lit        [][2]int // the pixels drawn
		alpha      int
	}{
		{"four pixels", 8, 8, 2, 2, [][2]int{{7, 7}, {8, 7}, {7, 8}, {8, 8}}, 255},
		{"a quarter of a pixel", 8.3, 8.3, 0.5, 0.5, [][2]int{{8, 8}}, 64},
		{"a quarter of a pixel wide, two high", 4.5, 8, 0.25, 2, [][2]int{{4, 7}, {4, 8}}, 64},
	} {
		pix := drawPoint(t, tc.x, tc.y, tc.w, tc.h, 0)
		drawn := map[[2]int]bool{}
		for _, p := range tc.lit {
			drawn[p] = true
		}
		for y := range 16 {
			for x := range 16 {
				p := pix(x, y)
				switch {
				case drawn[[2]int{x, y}] && (int(p[3]) < tc.alpha-2 || int(p[3]) > tc.alpha+2 ||
					p[0] != p[3]):
					t.Errorf("%s: pixel (%d, %d) is %v, want the sprite at alpha %d", tc.name, x, y,
						p, tc.alpha)
				case !drawn[[2]int{x, y}] && p[3] != 0:
					t.Errorf("%s: pixel (%d, %d) is %v, want nothing", tc.name, x, y, p)
				}
			}
		}
	}
}

func TestSprites_APointUnderAPixelReadsItsSpritesMiddle(t *testing.T) {
	// the pixel's middle lies by the point's top-left corner, where its sprite is red
	if p := drawPoint(t, 8.95, 8.95, 1, 1, 0)(8, 8); p != [4]byte{255, 255, 255, 255} {
		t.Errorf("the point is %v, want its sprite's white middle", p)
	}
}

func TestSprites_APointTurnsIntoTheFogByItsAmount(t *testing.T) {
	if p := drawPoint(t, 8, 8, 2, 2, 1)(8, 8); p != [4]byte{0, 0, 255, 255} {
		t.Errorf("a point all in the fog is %v, want the fog's blue", p)
	}
}
