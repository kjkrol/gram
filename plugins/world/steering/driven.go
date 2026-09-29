package steering

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Driven is an entity steered by hand: Ahead 1 to walk on the way it faces — Sprint urging it to
// its Steering's Sprint — -1 to brake to a stop and then back away facing as it does, 0 to let it
// go on as ordered or stand; Turn -1, 1 or 0 to turn anticlockwise, clockwise or not; Face, when
// not zero, a way to turn to face instead, whatever Turn says — where an eye riding in it looks.
// Flown, an entity that flies is flown by hand, as from inside it: it holds its height over sea
// level whatever the ground under it does, climbing and diving only along the way it is steered,
// which rises by Climb, -1 to 1, the sine of the rider's look up or down. Whoever steers it — a
// camera fastened to it, say — writes it every tick; the plugins that move entities carry it out,
// over the ground and, for the height, where the ground is known.
type Driven struct {
	Ahead, Turn int8
	Sprint      bool
	Face        geom.Vec
	Flown       bool
	Climb       float64
}

// Steepest is the sine of the steepest an entity flown by hand climbs or dives: 80°.
const Steepest = 0.985

// Slope is how the way a flown entity is steered along parts its speed: rise the share going up
// (down under zero), run the share along the ground; level, all along the ground, unless Flown.
func (d Driven) Slope() (rise, run float64) {
	if !d.Flown {
		return 0, 1
	}
	rise = min(max(d.Climb, -Steepest), Steepest)
	return rise, math.Sqrt(1 - rise*rise)
}
