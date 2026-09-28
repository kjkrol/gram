package render

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Benchmark_Composer_Render is the composer's own share of handing a frame to Ebitengine — the
// vertices gathered per sheet and indexed, the draw itself stubbed — for frames the size of the
// island's: a tile, an overlay on it and a line per cell in depth order on one sheet, a sprite of
// another sheet every 50 cells.
func Benchmark_Composer_Render(b *testing.B) {
	a, o := sheet{"atlas"}, sheet{"objects"}
	for _, n := range []int{6000, 24000} {
		c := NewComposer(items(func(f *Frame) {
			for i := range n {
				depth := float32(i)
				f.Sprite(Ground, depth, a, 0, unit, Even(1))
				f.Overlay(&Overlay{})
				f.Line(Overlays, depth, 0, 0, 1, 1, 1, white)
				if i%50 == 0 {
					f.Sprite(Objects, depth, o, 0, unit, Even(1))
				}
			}
		}))
		calls, verts := 0, 0
		c.draw = func(_ *ebiten.Image, v []ebiten.Vertex, _ []uint16, _ *ebiten.Image) { calls++; verts += len(v) }
		c.compose(sorted())
		b.Run(fmt.Sprintf("pieces=%d", c.Composed()), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				calls, verts = 0, 0
				c.render(nil)
			}
			b.ReportMetric(float64(calls), "calls")
			b.ReportMetric(float64(verts), "verts")
		})
	}
}
