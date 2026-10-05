package collision

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
)

// Band is a stretch of height, Bottom to Top: an entity's (its Z) or a solid cell's (from below
// up to its Height over the ground). Everywhere is every height: an entity without a Z or with no
// Height, a solid cell with no Height, a flat world — whatever does not say how high it is meets
// everything, as Layers 0 meets every plane. Only a world with heights asks.
type Band struct{ Bottom, Top float64 }

// Everywhere is the band of every height.
var Everywhere = Band{Bottom: math.Inf(-1), Top: math.Inf(1)}

// BandOf is the band an entity's Z spans, its bottom to its top; Everywhere for no Z or no Height.
func BandOf(z *world.Z) Band {
	if z == nil || z.Height <= 0 {
		return Everywhere
	}
	return Band{Bottom: z.Altitude, Top: z.Top()}
}

// Meets reports whether the two bands share a stretch of height; two meeting only at an edge do
// not (a crate on a platform).
func (b Band) Meets(o Band) bool { return b.Bottom < o.Top && o.Bottom < b.Top }
