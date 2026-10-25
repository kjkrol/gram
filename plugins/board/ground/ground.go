package ground

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
)

// Heights is the height of the board's ground at a point and how far apart it is sampled along a
// ray: a topography's relief; nil on a flat map.
type Heights interface {
	At(p geom.Vec) float32
	Step() float32
}

// Cover is what stands on the board and holds sight back, walked along a ray: Walk calls visit,
// nearest first, for every stretch of the ray from origin along the unit dir up to length inside a
// cell whose cover dims an observer on the layers blockers (zero: every layer), with the band it
// spans (±Inf for a cover of no height) and how see-through it is (tau; ≤ 0 blocks). visit returning false
// ends the walk.
type Cover interface {
	Walk(origin, dir geom.Vec, length float32, blockers world.Layers, visit func(near, far, bottom, top, tau float32) bool)
}

// Profile is Heights that knows its ground along a ray exactly: Along calls visit, nearest first,
// with every point from origin along the unit dir up to length where the ground's slope changes,
// and with length itself (how far along, and the height there); visit returning false ends the
// walk. Between those points the ground is straight.
type Profile interface {
	Along(origin, dir geom.Vec, length float32, visit func(d, alt float32) bool)
}

// Surfaces is where whoever moves in d with its feet at feet stands at p: the highest of the
// ground and a deck carrying d no higher than feet+step (floor), and the underside of a deck over
// its feet (ceiling, +Inf for none).
type Surfaces interface {
	Surface(p geom.Vec, d cell.Domain, feet, step float32) (floor, ceiling float32)
}

// Shaper is ground that can be shaped: Flatten lays the cell c flat at h, the ground round it
// following, and reports whether it did. It does nothing where a road would then lie where
// Transport Tycoon would not lay it (RoadFits).
type Shaper interface {
	Flatten(c cell.ID, h float32) bool
}
