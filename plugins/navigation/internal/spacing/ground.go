package spacing

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

const TargetWaitTimeout = 500 * time.Millisecond

// world units
const ArrivalEpsilon = 2.0

type Footing interface {
	CellFor(p geom.Vec, feet, reach float64) (cell.ID, bool)
	LevelFor(p geom.Vec, feet, reach float64) (c cell.ID, deck, ok bool)
	KindFor(c cell.ID, p geom.Vec, feet, reach float64) cell.Kind
}

// nil z: the ground's
func CellOf(g grid.Grid, t cell.Terrain, p geom.Vec, z *world.Z) (cell.ID, bool) {
	c, _, ok := LevelOf(g, t, p, z)
	return c, ok
}

func ShortestAxisDelta(have, want float64, size uint32, wraps bool) float64 {
	d := want - have
	if !wraps || size == 0 {
		return d
	}
	s := float64(size)
	if d > s/2 {
		d -= s
	} else if d < -s/2 {
		d += s
	}
	return d
}

func LevelOf(g grid.Grid, t cell.Terrain, p geom.Vec, z *world.Z) (c cell.ID, deck, ok bool) {
	f, known := t.(Footing)
	if !known {
		c, ok = g.CellAt(p)
		return c, false, ok
	}
	if z == nil {
		return f.LevelFor(p, math.Inf(-1), 0)
	}
	return f.LevelFor(p, z.Altitude, z.Reach())
}
