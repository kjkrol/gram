package main

import (
	"math"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// madeira is how the island is walked: the hawk flies over, a walker climbs or descends slopes
// up to 1 in 1 and the plateau's highlanders up to 2 in 1, the slope slowing all as Tobler says
type madeira struct{ cell.Hiking }

var _ cell.Mobility = madeira{}

const (
	walkerClimb     = 1.0
	highlanderClimb = 2.0
)

func (m madeira) Pace(s cell.Step) float64 {
	limit := walkerClimb
	if s.Domain&HighlandDomain != 0 {
		limit = highlanderClimb
	}
	if s.Domain&m.Free == 0 && math.Abs(s.Slope) > limit {
		return 0
	}
	return m.Hiking.Pace(s)
}
