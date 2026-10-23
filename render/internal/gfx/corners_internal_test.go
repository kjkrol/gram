package gfx

import "testing"

func TestFadeCustoms_StayUnderTheCapHoweverFarTheEdges(t *testing.T) {
	for _, size := range []float32{3, 400, 1e5, 1e7} {
		quad := Corners{{0, 0}, {size, 0}, {0, size}, {size, size}}
		for k, c := range FadeCustoms(quad, Fade{Left: 1, Right: 1, Top: 1, Bottom: 1}) {
			for _, v := range c {
				if v > softCap {
					t.Fatalf("corner %d of a quad %v wide fades %v, over the cap %v", k, size, v,
						softCap)
				}
			}
		}
	}
}
