package ground_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
)

func TestRoadFits(t *testing.T) {
	const run = 32
	g := grid.DefaultGrids{}.Square(3, 3, run)
	c := g.At(1, 1)
	way := func(dx, dy int) cell.Links {
		i, _ := grid.Direction(g, dx, dy)
		return 1 << i
	}
	n, s, w, e := way(0, -1), way(0, 1), way(-1, 0), way(1, 0)
	nw, ne, sw, se := way(-1, -1), way(1, -1), way(-1, 1), way(1, 1)
	steep := run * float32(math.Tan(ground.RoadSlope))
	for name, tc := range map[string]struct {
		corners [2][2]float32 // the top row's corners, then the bottom row's
		links   cell.Links
		want    bool
	}{
		"no road":                  {[2][2]float32{{0, 9}, {3, 1}}, 0, true},
		"level, a fork":            {[2][2]float32{{5, 5}, {5, 5}}, n | e | s, true},
		"level, an end":            {[2][2]float32{{5, 5}, {5, 5}}, w, true},
		"ramp north to south":      {[2][2]float32{{2, 2}, {5, 5}}, n | s, true},
		"ramp west to east":        {[2][2]float32{{2, 5}, {2, 5}}, w | e, true},
		"ramp, the road across it": {[2][2]float32{{2, 2}, {5, 5}}, w | e, false},
		"ramp, a bend":             {[2][2]float32{{2, 2}, {5, 5}}, n | e, false},
		"ramp, an end":             {[2][2]float32{{2, 2}, {5, 5}}, s, false},
		"ramp at the steepest":     {[2][2]float32{{0, 0}, {steep, steep}}, n | s, true},
		"ramp too steep": {
			[2][2]float32{{0, 0}, {steep + 1, steep + 1}}, n | s, false,
		},
		"uneven":                      {[2][2]float32{{2, 3}, {5, 5}}, n | s, false},
		"slantwise on a level cell":   {[2][2]float32{{4, 4}, {4, 4}}, nw | se, true},
		"slantwise on three level":    {[2][2]float32{{4, 4}, {9, 4}}, nw | se, false},
		"slantwise, the low one left": {[2][2]float32{{4, 1}, {4, 4}}, nw | se, false},
		"slantwise on two level":      {[2][2]float32{{4, 6}, {9, 4}}, nw | se, false},
		"slantwise across the slope":  {[2][2]float32{{4, 4}, {9, 4}}, ne | sw, false},
		"other diagonal, level":       {[2][2]float32{{4, 4}, {4, 4}}, ne | sw, true},
		"other diagonal":              {[2][2]float32{{7, 4}, {4, 4}}, ne | sw, false},
		"a bend onto a diagonal":      {[2][2]float32{{4, 4}, {9, 4}}, w | se, false},
	} {
		middle := g.CellCenter(c)
		var corners []float32
		for _, p := range g.CellOutline(c, nil) {
			corners = append(corners, tc.corners[below(p, middle)][right(p, middle)])
		}
		if got := ground.RoadFits(g, c, corners, tc.links, run); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
	if !ground.RoadJoins(12, 12) || ground.RoadJoins(12, 12.5) {
		t.Error("a road joins hexes of one level alone")
	}
}

func below(p, middle geom.Vec) int {
	if p.Y > middle.Y {
		return 1
	}
	return 0
}

func right(p, middle geom.Vec) int {
	if p.X > middle.X {
		return 1
	}
	return 0
}
