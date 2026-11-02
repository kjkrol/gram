package ground

import (
	"math"
	"sync"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// RoadSlope is the steepest a road climbs: an even ramp rises at most this much, in radians.
var RoadSlope = 10 * math.Pi / 180

// RoadFits reports whether a road running out of cell c of g as links say may lie on corners, the
// heights at c's CellOutline in its order, as Transport Tycoon lays it: level, or an even ramp it
// runs straight up across opposite edges, every corner on its plane, rising RoadSlope at most from
// the middle of one edge to the other's. A road through a corner, a bend, a fork or an end lies
// level, else a diagonal would lie lower on one side than on the other.
func RoadFits(g grid.Grid, c cell.ID, corners []float32, links cell.Links) bool {
	if links == 0 {
		return true
	}
	eps := 1e-4 * g.CellSpan()
	level := true
	for _, h := range corners[1:] {
		level = level && f32.Abs(h-corners[0]) <= eps
	}
	if level {
		return true
	}
	i, ok := grid.Straight(g, links)
	if !ok || g.Corner(i) {
		return false
	}
	return ramp(g, c, i, corners, eps)
}

// a stack array would escape through the grid's CellOutline
var outlines = sync.Pool{New: func() any { return new([8]geom.Vec) }}

// the corners lie on an even ramp no steeper than RoadSlope from the middle of the edge the road
// enters c by to the middle of the one it leaves it by, direction i
func ramp(g grid.Grid, c cell.ID, i int, corners []float32, eps float32) bool {
	from, to := grid.Exit(g, c, g.Opposite(i)), grid.Exit(g, c, i)
	ax, ay := to.X-from.X, to.Y-from.Y
	along := func(p geom.Vec) float32 {
		return ((p.X-from.X)*ax + (p.Y-from.Y)*ay) / (ax*ax + ay*ay)
	}
	buf := outlines.Get().(*[8]geom.Vec)
	defer outlines.Put(buf)
	outline := g.CellOutline(c, buf[:0])
	enter, leave := 0, 0
	for k, p := range outline {
		switch t := along(p); {
		case t < along(outline[enter]):
			enter = k
		case t > along(outline[leave]):
			leave = k
		}
	}
	same := func(a, b float32) bool { return f32.Abs(a-b) <= eps }
	h0, h1 := corners[enter], corners[leave]
	fits := f32.Abs(h1-h0) <= f32.Hypot(ax, ay)*float32(math.Tan(RoadSlope))+eps
	for k, p := range outline {
		t := along(p)
		// exactly h0 and h1 at the ends, which h0+(h1-h0)*t is not
		fits = fits && same(corners[k], h0*(1-t)+h1*t)
	}
	return fits
}
