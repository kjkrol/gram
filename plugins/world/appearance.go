package world

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// angleOf is the engine's one angle convention — degrees, 0 east, against the clock with the
// screen's y growing down (render.Arrow draws by it): the way d heads, for a turned sprite.
func angleOf(d geom.Vec) float32 {
	return float32(math.Atan2(-d.Y, d.X) * 180 / math.Pi)
}

// headingIndex is which of n ways round the circle d heads, east first against the clock: the
// index of a directional twin drawn at i*360/n degrees.
func headingIndex(d geom.Vec, n int) int {
	i := int(math.Round(math.Atan2(-d.Y, d.X) / (2 * math.Pi) * float64(n)))
	return ((i % n) + n) % n
}
