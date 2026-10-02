package relief

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// MeanOfCells is a height function for SetHeights built from one height per cell: a point inside
// a cell is at its height, a point where cells meet at the mean of theirs.
func MeanOfCells(grid grid.Grid, height func(c cell.ID) float64) func(p geom.Vec) float64 {
	const eps = 1e-6
	return func(p geom.Vec) float64 {
		var seen [4]cell.ID
		n, sum := 0, 0.0
	next:
		for _, d := range [4][2]float64{{-eps, -eps}, {eps, -eps}, {-eps, eps}, {eps, eps}} {
			c, ok := grid.CellAt(geom.NewVec(p.X+d[0], p.Y+d[1]))
			if !ok {
				continue
			}
			for _, s := range seen[:n] {
				if s == c {
					continue next
				}
			}
			seen[n], n, sum = c, n+1, sum+height(c)
		}
		if n == 0 {
			return 0
		}
		return sum / float64(n)
	}
}
