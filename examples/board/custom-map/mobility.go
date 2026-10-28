package main

import "github.com/kjkrol/gram/plugins/board/cell"

// hillside is how the walkers fare on the hills: a step uphill dearer the steeper it climbs, a
// step down as quick as the flat
type hillside struct{}

var _ cell.Mobility = hillside{}

const climbCost = 4 // how many times dearer a step is for every unit of rise over run

func (hillside) Pace(s cell.Step) float64 {
	return s.Kind.PaceFor(s.Domain) / (1 + climbCost*max(s.Slope, 0))
}
