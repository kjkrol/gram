package steering

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
)

// Driven is an entity steered by hand, written every tick by whoever steers it. Ahead is 1 to
// walk on, -1 to brake and back away, 0 neither; Turn -1 anticlockwise, 1 clockwise; Side 1 to
// strafe right, -1 left, facing on; Face a way to turn to instead, Look the way an eye riding
// in it looks, turned to before Face; Flown, it holds its height, rising by Climb (-1 to 1, the
// sine of the look up or down) along its way; Held how long the hand has lain on it without a
// break.
type Driven struct {
	Ahead, Turn int8
	Side        int8
	Sprint      bool
	Face        geom.Vec
	Look        geom.Vec
	Flown       bool
	Climb       float64
	Held        time.Duration
}

// sin 80°, the steepest climb or dive
const steepest = 0.985

// Slope is how the way a flown entity is steered parts its speed: rise the share going up, run
// the share along the ground; not Flown, all along the ground.
func (d Driven) Slope() (rise, run float64) {
	if !d.Flown {
		return 0, 1
	}
	rise = min(max(d.Climb, -steepest), steepest)
	return rise, math.Sqrt(1 - rise*rise)
}

// Steers reports whether anything steers the entity this step.
func (d Driven) Steers() bool {
	none := geom.Vec{}
	return d.Ahead != 0 || d.Turn != 0 || d.Side != 0 || d.Face != none || d.Look != none
}
