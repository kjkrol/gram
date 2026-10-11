package ground_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
)

const (
	n cell.Links = 1 << iota
	s
	w
	e
	nw
	ne
	sw
	se
)

// A road lies on a level cell any way; on an even ramp only straight up or down it, no steeper
// than RoadSlope; slantwise along a level diagonal with a third corner beside it.
func TestRoadFits(t *testing.T) {
	const run = 32
	steep := run * math.Tan(ground.RoadSlope)
	for name, c := range map[string]struct {
		corners [4]float64
		links   cell.Links
		want    bool
	}{
		"no road":                     {[4]float64{0, 9, 3, 1}, 0, true},
		"level, a fork":               {[4]float64{5, 5, 5, 5}, n | e | s, true},
		"level, an end":               {[4]float64{5, 5, 5, 5}, w, true},
		"ramp north to south":         {[4]float64{2, 2, 5, 5}, n | s, true},
		"ramp west to east":           {[4]float64{2, 5, 2, 5}, w | e, true},
		"ramp, the road across it":    {[4]float64{2, 2, 5, 5}, w | e, false},
		"ramp, a bend":                {[4]float64{2, 2, 5, 5}, n | e, false},
		"ramp, an end":                {[4]float64{2, 2, 5, 5}, s, false},
		"ramp at the steepest":        {[4]float64{0, 0, steep, steep}, n | s, true},
		"ramp too steep":              {[4]float64{0, 0, steep + 1, steep + 1}, n | s, false},
		"uneven":                      {[4]float64{2, 3, 5, 5}, n | s, false},
		"slantwise on three level":    {[4]float64{4, 4, 9, 4}, nw | se, true},
		"slantwise, the low one left": {[4]float64{4, 1, 4, 4}, nw | se, true},
		"slantwise on two level":      {[4]float64{4, 6, 9, 4}, nw | se, false},
		"slantwise across the slope":  {[4]float64{4, 4, 9, 4}, ne | sw, false},
		"other diagonal":              {[4]float64{7, 4, 4, 4}, ne | sw, true},
		"a bend onto a diagonal":      {[4]float64{4, 4, 9, 4}, w | se, false},
	} {
		if got := ground.RoadFits(c.corners, c.links, run); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	if !ground.RoadJoins(12, 12) || ground.RoadJoins(12, 12.5) {
		t.Error("a road joins hexes of one level alone")
	}
}
