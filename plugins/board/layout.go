package board

import "github.com/kjkrol/aabbworld/geom"

// Layout is a board's initial terrain for Plugin.Seed: Default fills every cell, then each
// CellEntry overrides one; each WayEntry lays a Way across a cell; Heights, when set, raises the
// ground — see Board.SetHeights.
type Layout struct {
	Default string
	Cells   []CellEntry
	Ways    []WayEntry
	Heights func(p geom.Vec) float64
}

// WayEntry lays a Way of the CellKind named Kind across Cell, Width wide, running on as Links says.
type WayEntry struct {
	Kind  string
	Cell  CellID
	Width float32
	Links Links
}

// CellEntry sets Cell to the CellKind named Kind.
type CellEntry struct {
	Kind string
	Cell CellID
}

// MeanOfCells is a height function for Layout.Heights built from one height per cell: a point
// inside a cell is at its height, a point where cells meet at the mean of theirs.
func MeanOfCells(grid Grid, height func(c CellID) float64) func(p geom.Vec) float64 {
	const eps = 1e-6
	return func(p geom.Vec) float64 {
		var seen [4]CellID
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
