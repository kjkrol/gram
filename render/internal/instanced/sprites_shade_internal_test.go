package instanced

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render/internal/atlas"
	"github.com/kjkrol/gram/render/internal/compose"
	"github.com/kjkrol/gram/render/internal/gfx"
)

// the ground's shade dark everywhere under the relief's shadow reaching span over it, no block's
func darkGround(span float32) map[string]any {
	dark := gfx.NewImage(1, 1)
	dark.Fill(color.RGBA{R: 0, G: 255, B: 255, A: 255})
	return map[string]any{
		"GroundShade":     dark,
		"GroundShadeWas":  dark,
		"GroundShadeArea": []float32{0, 0, 64, 64},
		"GroundShadeSpan": []float32{span, 1},
		"GroundShadeMix0": []float32{1, 1, 1, 1},
		"GroundShadeMix1": []float32{1, 1, 1, 1},
		"GroundShadeMix2": []float32{1, 1, 1, 1},
		"GroundShadeMix3": []float32{1, 1, 1, 1},
	}
}

func TestSprites_TakeTheShadeTheGroundHoldsAsTheirBitsSay(t *testing.T) {
	gputest.Need(t)
	sheet := atlas.NewAtlas()
	sheet.Add(SpriteID(1), 8, solid(white))
	sheet.Close()
	cam := cameratest.NewFromSpace(64, 64, 0)
	cam.SetViewport(64, 64)
	draw := func(takes Taking, sun float32) byte {
		var s Sprites
		s.Take(takes)
		s.Direct(sun)
		s.Rect(sheet, 1, 16, 16, 48, 48, 0, 0, 1, 1, Light{1, 1, 1})
		screen := gfx.NewImage(64, 64)
		s.Draw(Target{Screen: screen}, cam, compose.UniformsOf(darkGround(100)))
		pix := make([]byte, 4*64*64)
		screen.ReadPixels(pix)
		return pix[4*(32*64+32)]
	}
	if got := draw(TakesAll, 1); got > 10 {
		t.Errorf("a sprite taking all is lit %d in the shade, want dark", got)
	}
	if got := draw(TakesAll, 0.5); got < 118 || got > 138 {
		t.Errorf("a sprite half lit by the sun is lit %d in the shade, want about half", got)
	}
	if got := draw(0, 1); got < 250 {
		t.Errorf("a sprite taking nothing is lit %d in the shade, want bright", got)
	}
	if got := draw(compose.TakesClouds|compose.TakesLamps, 1); got < 250 {
		t.Errorf("a sprite taking neither shadow is lit %d in the shade, want bright", got)
	}
	if got := draw(compose.TakesRelief, 1); got > 10 {
		t.Errorf("a sprite taking the relief's shadow is lit %d in the shade, want dark", got)
	}
	if got := draw(compose.TakesBlocks, 1); got < 250 {
		t.Errorf("a sprite taking the blocks' shadow alone is lit %d in the relief's shade, want "+
			"bright", got)
	}
}
