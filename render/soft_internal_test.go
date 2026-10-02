package render

import (
	"image/color"
	"testing"
)

// However large a softly faded piece — one thrown millions of pixels off by a perspective — its
// fades never pass for a blended sprite's mark, which the shaders tell apart at half of it; a
// blended sprite's always does.
func TestSoft_NeverPassesForABlendedSprite(t *testing.T) {
	var f Frame
	f.Reset(nil)
	for _, size := range []float32{3, 400, 1e5, 1e7} {
		f.Soft(Air, 0, Corners{{0, 0}, {size, 0}, {0, size}, {size, size}}, color.RGBA{A: 200}, Fade{Left: 1, Right: 1, Top: 1, Bottom: 1})
	}
	f.Each(func(_ Tier, _ float32, v []Vertex) {
		for _, x := range v {
			for _, c := range []float32{x.Custom0, x.Custom1, x.Custom2, x.Custom3} {
				if c > softCap || c > blendMark/2 {
					t.Fatalf("a soft piece's fade reads %v, over the cap %v: it would pass for a blended sprite", c, softCap)
				}
			}
		}
	})
	var g Frame
	g.Reset(nil)
	g.SpriteBlend(Ground, 0, blendSheet{}, 0, Corners{{0, 0}, {8, 0}, {0, 8}, {8, 8}}, Even(1), [4]float32{0, 1, 0, 1}, 0.2)
	g.Each(func(_ Tier, _ float32, v []Vertex) {
		for _, x := range v {
			if x.Custom3 <= blendMark/2 || x.Custom3 > blendMark+0.5 {
				t.Errorf("a blended sprite's mark reads %v, want over half the mark %v", x.Custom3, blendMark/2)
			}
		}
	})
	// a piece a few fades long fades as before: 1 on the faded side, 1 plus its length in fades
	var h Frame
	h.Reset(nil)
	h.Soft(Air, 0, Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}, color.RGBA{A: 200}, Fade{Bottom: 4})
	h.Each(func(_ Tier, _ float32, v []Vertex) {
		if v[0].Custom3 != 1+20.0/4 || v[2].Custom3 != 1 {
			t.Errorf("a piece 20 tall fading over 4 at its bottom reads %v at the top and %v at the bottom, want 6 and 1", v[0].Custom3, v[2].Custom3)
		}
	})
}

// blendSheet is an AtlasSource of one sprite with no image behind it.
type blendSheet struct{}

func (blendSheet) Atlas() *Image                            { return nil }
func (blendSheet) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (blendSheet) White() (u, v float32)                    { return 9, 9 }

// A plain sprite far off in the air reads how much of it the air hides in its alpha, over 1 and
// under an overlay's mark; a sprite that is no plain one is left alone.
func TestFog_TheSpriteReadsItInItsAlphaUnderTheOverlaysMark(t *testing.T) {
	var f Frame
	f.Reset(nil)
	f.Sprite(Ground, 0, blendSheet{}, 0, Corners{{0, 0}, {8, 0}, {0, 8}, {8, 8}}, Even(1))
	f.Fog([4]float32{0, 0.5, 1, 2})
	f.Each(func(_ Tier, _ float32, v []Vertex) {
		if v[0].ColorA != 1 || v[1].ColorA != 1+fogSpan/2 || v[3].ColorA != 1+fogSpan || v[3].ColorA >= overlayMark-0.5 {
			t.Errorf("the fogged sprite's alphas are %v %v %v %v, want 1, 1 + half the span, 1 + all of it, under the overlays' 1.5", v[0].ColorA, v[1].ColorA, v[2].ColorA, v[3].ColorA)
		}
	})
	var g Frame
	g.Reset(nil)
	g.SpriteBlend(Ground, 0, blendSheet{}, 0, Corners{{0, 0}, {8, 0}, {0, 8}, {8, 8}}, Even(1), [4]float32{0, 1, 0, 1}, 0.2)
	g.Fog([4]float32{1, 1, 1, 1})
	g.Each(func(_ Tier, _ float32, v []Vertex) {
		if v[1].ColorA != 1 {
			t.Errorf("a blended sprite fogged reads alpha %v, want its weight left alone", v[1].ColorA)
		}
	})
}
