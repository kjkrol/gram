package gfx

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/gram/internal/gpu/gputest"
)

func TestImage_DrawImageWithAFadeFeathersEachEdge(t *testing.T) {
	gputest.Need(t)
	src := NewImage(64, 64)
	src.Fill(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	dst := NewImage(64, 64)
	dst.Clear()
	dst.DrawImage(src, &DrawImageOptions{Fade: Fade{Left: 16, Right: 16, Top: 16, Bottom: 16}})
	pix := make([]byte, 4*64*64)
	dst.ReadPixels(pix)
	// how much shows at the centre of the pixel px pixels in from an edge fading over 16
	smooth := func(px float64) float64 {
		k := min(max((px+0.5)/16, 0), 1)
		return k * k * (3 - 2*k)
	}
	for _, c := range []struct {
		x, y int
		want float64
		why  string
	}{
		{32, 32, 1, "the middle, past every fade"},
		{0, 32, smooth(0), "the left edge"},
		{8, 32, smooth(8), "halfway into the left fade"},
		{8, 8, smooth(8) * smooth(8), "halfway into the left and the top fades: their product"},
	} {
		at := 4 * (c.y*64 + c.x)
		got, want := int(pix[at+3]), int(math.Round(255*c.want))
		if got < want-3 || got > want+3 || int(pix[at]) != got {
			t.Errorf("at (%d, %d) the pixel is %v, want alpha %d and white premultiplied by it: %s",
				c.x, c.y, pix[at:at+4], want, c.why)
		}
	}
}
