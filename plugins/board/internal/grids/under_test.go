package grids

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// The square grid walks the cells under a box by arithmetic; the oracle is the lattice of any grid
// over the box and its images a lap away round each wrapping axis, edges touched and boxes off the
// board included.
func TestSquareGrid_CellsUnderIsWhatTheLatticeFinds(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	for _, wrap := range [][2]bool{{false, false}, {true, false}, {true, true}} {
		g := &Square{Width: 7, Height: 5, CellSize: 10, WrapX: wrap[0], WrapY: wrap[1]}
		boxes := []geom.AABB{
			geom.NewAABBAt(geom.NewVec(10, 10), 10, 10),  // one cell, its edges on the lines
			geom.NewAABBAt(geom.NewVec(-15, -5), 30, 12), // off the top left
			geom.NewAABBAt(geom.NewVec(-40, 3), 200, 4),  // wider than the board
			geom.NewAABBAt(geom.NewVec(65, 45), 20, 20),  // off the bottom right
		}
		for range 200 {
			boxes = append(boxes, geom.NewAABBAt(geom.NewVec(rng.Float64()*120-30, rng.Float64()*90-20), rng.Float64()*40, rng.Float64()*40))
		}
		for _, box := range boxes {
			var got, want []cell.ID
			g.CellsUnder(box, func(c cell.ID) { got = append(got, c) })
			want = latticeUnder(g, box)
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Fatalf("wrap %v, box %v: cells %v, want %v", wrap, box, got, want)
			}
		}
	}
}

// latticeUnder is every cell cellsUnder finds under box or any image of it a whole lap away.
func latticeUnder(g *Square, box geom.AABB) []cell.ID {
	laps := func(wraps bool) int {
		if wraps {
			return 4
		}
		return 0
	}
	w, h := float64(g.Width*g.CellSize), float64(g.Height*g.CellSize)
	var out []cell.ID
	for sy := -laps(g.WrapY); sy <= laps(g.WrapY); sy++ {
		for sx := -laps(g.WrapX); sx <= laps(g.WrapX); sx++ {
			shift := geom.NewVec(float64(sx)*w, float64(sy)*h)
			image := geom.NewAABB(box.TopLeft.Add(shift), box.BottomRight.Add(shift))
			cellsUnder(g, image, func(c cell.ID) {
				if !slices.Contains(out, c) {
					out = append(out, c)
				}
			})
		}
	}
	slices.Sort(out)
	return out
}
