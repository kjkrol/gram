package world

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// degrees, 0 east, against the clock with the screen's y down (render.Arrow's convention)
func angleOf(d geom.Vec) float32 {
	return float32(math.Atan2(-d.Y, d.X) * 180 / math.Pi)
}

// east first against the clock: twin i is drawn at i*360/n degrees
func headingIndex(d geom.Vec, n int) int {
	i := int(math.Round(math.Atan2(-d.Y, d.X) / (2 * math.Pi) * float64(n)))
	return ((i % n) + n) % n
}
