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
		if got := ground.RoadFits(g, c, corners, tc.links); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
}

func TestRoadFits_Hex(t *testing.T) {
	const size = 10
	g := grid.DefaultGrids{}.Hex(3, 3, size)
	c := g.At(1, 1)
	way := func(dq, dr int) cell.Links {
		i, _ := grid.Direction(g, dq, dr)
		return 1 << i
	}
	e, ne, nw, w, sw, se := way(1, 0), way(1, -1), way(0, -1), way(-1, 0), way(-1, 1), way(0, 1)
	// the run from an edge's middle to the opposite one's is √3 sizes
	steep := float32(math.Sqrt(3)) * size * float32(math.Tan(ground.RoadSlope))
	east := func(rise float32) [6]float32 {
		return [6]float32{rise / 2, rise, rise, rise / 2, 0, 0}
	}
	for name, tc := range map[string]struct {
		corners [6]float32 // clockwise from the top, as CellOutline lays them
		links   cell.Links
		want    bool
	}{
		"no road":                       {[6]float32{0, 9, 3, 1, 4, 2}, 0, true},
		"level, a fork":                 {[6]float32{5, 5, 5, 5, 5, 5}, e | nw | sw, true},
		"level, an end":                 {[6]float32{5, 5, 5, 5, 5, 5}, w, true},
		"ramp west to east":             {east(2), e | w, true},
		"ramp south-west to north-east": {[6]float32{2, 2, 1, 0, 0, 1}, ne | sw, true},
		"ramp north-west to south-east": {[6]float32{0, 1, 2, 2, 1, 0}, nw | se, true},
		"ramp, the road across it":      {east(2), ne | sw, false},
		"ramp, a bend":                  {east(2), e | nw, false},
		"ramp, an end":                  {east(2), w, false},
		"ramp at the steepest":          {east(steep), e | w, true},
		"ramp too steep":                {east(steep + 1), e | w, false},
		"a side corner off the ramp":    {[6]float32{1.5, 2, 2, 1, 0, 0}, e | w, false},
		"the side corners low":          {[6]float32{0, 2, 2, 0, 0, 0}, e | w, false},
	} {
		if got := ground.RoadFits(g, c, tc.corners[:], tc.links); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
}

func TestRoadFits_AllocatesNothing(t *testing.T) {
	for _, g := range []grid.Grid{
		grid.DefaultGrids{}.Square(3, 3, 32),
		grid.DefaultGrids{}.Hex(3, 3, 10),
	} {
		c := g.At(1, 1)
		i, _ := grid.Direction(g, 1, 0)
		links := cell.Links(1<<i | 1<<g.Opposite(i))
		middle := g.CellCenter(c)
		var corners []float32
		for _, p := range g.CellOutline(c, nil) {
			corners = append(corners, (p.X-middle.X)/10)
		}
		if !ground.RoadFits(g, c, corners, links) {
			t.Fatalf("a ramp rising east %v refused", corners)
		}
		check := func() { ground.RoadFits(g, c, corners, links) }
		if n := testing.AllocsPerRun(100, check); n != 0 {
			t.Errorf("a ramp of %d corners checked allocates %v, want nothing", len(corners), n)
		}
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
