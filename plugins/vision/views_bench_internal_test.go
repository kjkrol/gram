package vision

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
)

// Benchmark_Views_Copy is one frame's copy of the ground and the cover for the views, over a
// 192x96 board of 32-px cells: in relief and flat, warm (nothing changed since the last frame)
// and with a cell changed every frame.
func Benchmark_Views_Copy(b *testing.B) {
	const cols, rows, size = 192, 96, 32
	for _, sc := range []struct {
		name   string
		relief bool
	}{{"relief", true}, {"flat", false}} {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: cols * size, Height: rows * size},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
			Heights:  true,
		})
		g := grid.DefaultGrids{}.Square(cols, rows, size)
		brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
		var heights ground.Heights
		if sc.relief {
			heights = topography.NewPlugin(w, brd, nil, topography.Config{}).Heights()
		}
		plain := cell.Kind{Cost: 1, Allows: cell.Land}
		forest := cell.Kind{Cost: 1, Allows: cell.Land, Veil: 0.5, Veils: cell.Land}
		brd.Write().Board().SetAll(plain)
		for x := range uint32(cols) {
			brd.Write().Board().Set(g.At(x, 40), forest)
		}
		cover := brd.Read(0).Cover()
		far := g.At(10, 10)
		for _, changed := range []bool{false, true} {
			name := sc.name + ",warm"
			if changed {
				name = sc.name + ",changed"
			}
			b.Run(name, func(b *testing.B) {
				v := newViews()
				v.copy(heights, cover, cols*size, rows*size, size, [2]bool{})
				b.ReportAllocs()
				for i := 0; b.Loop(); i++ {
					if changed {
						if i%2 == 0 {
							brd.Write().Board().Set(far, forest)
						} else {
							brd.Write().Board().Set(far, plain)
						}
					}
					v.copy(heights, cover, cols*size, rows*size, size, [2]bool{})
				}
			})
		}
	}
}
