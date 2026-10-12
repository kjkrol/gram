package collision

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
)

// Band is a stretch of height, Bottom to Top, an entity's or a solid cell's; Step is how high an
// entity steps up. Whatever does not say how high it is spans Everywhere.
type Band struct{ Bottom, Top, Step float64 }

// Everywhere is the band of every height.
var Everywhere = Band{Bottom: math.Inf(-1), Top: math.Inf(1)}

// BandOf is the band an entity's Z spans, its bottom to its top; Everywhere for no Z or no Height.
func BandOf(z *world.Z) Band {
	if z == nil || z.Height <= 0 {
		return Everywhere
	}
	return Band{Bottom: z.Altitude, Top: z.Top(), Step: z.Reach()}
}

// Above is the band over b's step: what of it a deck's slab strikes.
func (b Band) Above() Band { return Band{Bottom: b.Bottom + b.Step, Top: b.Top} }

// Meets reports whether the two bands share a stretch of height; two meeting only at an edge do
// not (a crate on a platform).
func (b Band) Meets(o Band) bool { return b.Bottom < o.Top && o.Bottom < b.Top }
