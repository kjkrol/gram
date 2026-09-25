package world

import (
	"math"
	"testing"
)

func TestUVSpan_PiecesTileTheTextureAcrossTheCorner(t *testing.T) {
	const sprite, shifted = 100, -1000

	headU0, headU1 := uvSpan(30, sprite, 0)
	tailU0, tailU1 := uvSpan(70, sprite, shifted)
	headV0, headV1 := uvSpan(40, sprite, 0)
	tailV0, tailV1 := uvSpan(60, sprite, shifted)

	for _, tc := range []struct {
		axis           string
		h0, h1, t0, t1 float32
	}{
		{"across", headU0, headU1, tailU0, tailU1},
		{"down", headV0, headV1, tailV0, tailV1},
	} {
		if tc.h0 != 0 {
			t.Errorf("%s: the head starts at %v, want 0", tc.axis, tc.h0)
		}
		if tc.t1 != 1 {
			t.Errorf("%s: the tail ends at %v, want 1", tc.axis, tc.t1)
		}
		if math.Abs(float64(tc.h1-tc.t0)) > 1e-6 {
			t.Errorf("%s: the head ends at %v but the tail starts at %v — the pieces do not meet", tc.axis, tc.h1, tc.t0)
		}
	}

	if _, u1 := uvSpan(70, sprite, shifted); u1 != 1 {
		t.Errorf("the right piece ends at %v across, want 1", u1)
	}
	if v0, _ := uvSpan(40, sprite, 0); v0 != 0 {
		t.Errorf("the right piece starts at %v downwards, want 0 — it never wrapped on that axis", v0)
	}
}

// A sprite that never reaches an edge shows all of itself.
func TestUVSpan_UnwrappedSpriteShowsWholeTexture(t *testing.T) {
	u0, u1 := uvSpan(100, 100, 0)
	if u0 != 0 || u1 != 1 {
		t.Errorf("u = [%v, %v], want the whole sprite [0, 1]", u0, u1)
	}
}
