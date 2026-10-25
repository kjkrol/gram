package ground

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// MeanOfCells is the ground's heights built from one height per cell, for a topography's Seed: a
// point inside a cell is at its height, a point where cells meet at the mean of theirs.
func MeanOfCells(grid grid.Grid, height func(c cell.ID) float32) func(p geom.Vec) float32 {
	return func(p geom.Vec) float32 {
		var seen [4]cell.ID
		n, sum := 0, 0.0
		eps := f32.Eps(max(f32.Abs(p.X), f32.Abs(p.Y)))
	next:
		for _, d := range [4][2]float32{{-eps, -eps}, {eps, -eps}, {-eps, eps}, {eps, eps}} {
			c, ok := grid.CellAt(geom.NewVec(p.X+d[0], p.Y+d[1]))
			if !ok {
				continue
			}
			for _, s := range seen[:n] {
				if s == c {
					continue next
				}
			}
			seen[n], n, sum = c, n+1, sum+float64(height(c))
		}
		if n == 0 {
			return 0
		}
		return float32(sum / float64(n))
	}
}
