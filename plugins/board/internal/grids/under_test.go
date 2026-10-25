package grids

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

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
			boxes = append(boxes, geom.NewAABBAt(geom.NewVec(float32(rng.Float64()*120-30), float32(rng.Float64()*90-20)),
				float32(rng.Float64()*40),
				float32(rng.Float64()*40),
			),
			)
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

func latticeUnder(g *Square, box geom.AABB) []cell.ID {
	laps := func(wraps bool) int {
		if wraps {
			return 4
		}
		return 0
	}
	w, h := float32(g.Width*g.CellSize), float32(g.Height*g.CellSize)
	var out []cell.ID
	for sy := -laps(g.WrapY); sy <= laps(g.WrapY); sy++ {
		for sx := -laps(g.WrapX); sx <= laps(g.WrapX); sx++ {
			shift := geom.NewVec(float32(sx)*w, float32(sy)*h)
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
