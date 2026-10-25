package world

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
)

// degrees, 0 east, against the clock with the screen's y down (render.Arrow's convention)
func angleOf(d geom.Vec) float32 {
	return f32.Atan2(-d.Y, d.X) * 180 / math.Pi
}

// east first against the clock: twin i is drawn at i*360/n degrees
func headingIndex(d geom.Vec, n int) int {
	i := int(f32.Round(f32.Atan2(-d.Y, d.X) / (2 * math.Pi) * float32(n)))
	return ((i % n) + n) % n
}
