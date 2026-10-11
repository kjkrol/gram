package ground

import (
	"math"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// RoadSlope is the steepest a road climbs: an even ramp rises at most this much, in radians.
var RoadSlope = 10 * math.Pi / 180

// The ways out of a square cell, as cell.Links names them.
const (
	north cell.Links = 1 << iota
	south
	west
	east
	northWest
	northEast
	southWest
	southEast
)

// RoadFits reports whether a road running out of a square cell as links say may lie on corners —
// top-left, top-right, bottom-left, bottom-right — run the cell's side, as Transport Tycoon lays
// it: on a level cell any way; on an even ramp, two corners at one height and the two across at
// another no more than RoadSlope higher, only straight up or down it; slantwise only along a
// diagonal whose ends stand level with a third corner. A bend, a fork or an end lies level.
func RoadFits(corners [4]float64, links cell.Links, run float64) bool {
	if links == 0 {
		return true
	}
	eps := 1e-4 * run
	same := func(a, b float64) bool { return math.Abs(a-b) <= eps }
	tl, tr, bl, br := corners[0], corners[1], corners[2], corners[3]
	if same(tl, tr) && same(tl, bl) && same(tl, br) {
		return true
	}
	climbs := func(a, b float64) bool { return math.Abs(a-b) <= run*math.Tan(RoadSlope)+eps }
	switch links {
	case north | south:
		return same(tl, tr) && same(bl, br) && climbs(tl, bl)
	case west | east:
		return same(tl, bl) && same(tr, br) && climbs(tl, tr)
	case northWest | southEast:
		return same(tl, br) && (same(tr, tl) || same(bl, tl))
	case northEast | southWest:
		return same(tr, bl) && (same(tl, tr) || same(br, tr))
	}
	return false
}

// RoadJoins reports whether a road joins two cells of a grid of level cells — a hex grid — standing
// at levels a and b: only level with each other.
func RoadJoins(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6*max(1, math.Abs(a), math.Abs(b))
}
