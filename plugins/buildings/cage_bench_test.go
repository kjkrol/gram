package buildings

import (
	"math/rand/v2"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

// a town as Madeira's: a house 29 by 19 on each of the cells of a board of 64 x 64 cells of 32,
// turned by quarters, on ground rising across it to 40, under gables; points over it from under
// the ground to over the roofs
func townCage(b *testing.B) (*Plugin, [][3]float32) {
	b.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 64 * 32, Height: 64 * 32},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 64},
		Heights:  true,
	})
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(64, 64, 32), &cell.MultipleOccupancy{}, w)
	p := NewPlugin(w, brd)
	spec := Spec{Storey: 14, Floors: 2, Parts: []Part{{Roof: Gable, Pitch: 0.5}}}
	ground := func(q geom.Vec) float32 { return q.X * 40 / (64 * 32) }
	c := &p.cage
	c.stale, c.cells, c.low, c.high = false, map[cell.ID][]int32{}, 0, 100
	cellAt := brd.Read(0).Board().CellAt
	for y := range 64 {
		for x := range 64 {
			at := spec.of(Row{Mid: [2]float32{float32(x)*32 + 16, float32(y)*32 + 16},
				Size: [2]float32{29, 19}, Turn: float32(90 * ((x + y) % 4))})
			for _, part := range heldOf(spec, at, ground, nil, -1) {
				c.parts = append(c.parts, part)
				c.index(cellAt, 32, int32(len(c.parts)-1))
			}
		}
	}
	r := rand.New(rand.NewPCG(1, 2))
	pts := make([][3]float32, 4096)
	for i := range pts {
		pts[i] = [3]float32{r.Float32() * 64 * 32, r.Float32() * 64 * 32, r.Float32() * 100}
	}
	return p, pts
}

func Benchmark_Buildings_Free(b *testing.B) {
	p, pts := townCage(b)
	for b.Loop() {
		for _, q := range pts {
			p.read.FreeTo(camera.Behind, q[0], q[1], q[2])
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(pts)), "ns/point")
}
