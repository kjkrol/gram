package steering

import "github.com/kjkrol/aabbworld/geom"

// Driven is an entity steered by hand: Ahead 1 to walk on the way it faces, -1 to stop, 0 to let
// it go on as ordered or stand; Turn -1, 1 or 0 to turn anticlockwise, clockwise or not; Face,
// when not zero, a way to turn to face instead, whatever Turn says — where an eye riding in it
// looks. Whoever steers it — a camera fastened to it, say — writes it every tick; the plugin that
// moves entities over the ground carries it out, keeping them where they may go.
type Driven struct {
	Ahead, Turn int8
	Face        geom.Vec
}
