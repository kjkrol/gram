package render

import (
	"fmt"
	"testing"
)

// The draw itself is stubbed: the composer's own share of a frame the size of the island's.
func Benchmark_Composer_Render(b *testing.B) {
	a, o := sheet{"atlas"}, sheet{"objects"}
	for _, n := range []int{6000, 24000} {
		c := NewComposer(items(func(f *Frame) {
			for i := range n {
				depth := float32(i)
				f.Sprite(h(depth), a, 0, unit, Even(1))
				f.Overlay(&Overlay{})
				f.Line(h(depth), 0, 0, 1, 1, 1, white)
				if i%50 == 0 {
					f.Sprite(h(depth), o, 0, unit, Even(1))
				}
			}
		}))
		calls, verts := 0, 0
		c.draw = func(_ *Image, v []Vertex, _ []uint16, _ *Image, _ [4]float32) {
			calls++
			verts += len(v)
		}
		c.compose(sorted(), wholePart)
		b.Run(fmt.Sprintf("pieces=%d", c.frame.Len()), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				calls, verts = 0, 0
				c.render(nil, false, 0)
			}
			b.ReportMetric(float64(calls), "calls")
			b.ReportMetric(float64(verts), "verts")
		})
	}
}
