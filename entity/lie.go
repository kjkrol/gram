package entity

import "github.com/kjkrol/aabbworld/geom"

// Lie is how an entity lying flat lies: a rectangle Length along its heading (Velocity.Dir) and
// Width across, its middle Ahead of its box's along the heading, level at its altitude; Edged,
// edges stand along its long sides Edge high — a bridge's deck and its railings. Zero, it lies
// over its box.
type Lie struct {
	Length, Width float64
	Ahead         float64
	Edged         bool
	Edge          float64
}

// Corners are the corners of the rectangle the entity lies over, its box's middle at middle and
// headed dir, as a sprite authored facing east is laid over it: top-left, top-right, bottom-left,
// bottom-right.
func (l Lie) Corners(middle, dir geom.Vec) [4]geom.Vec {
	if dir == (geom.Vec{}) {
		dir = geom.NewVec(1, 0)
	}
	middle = geom.NewVec(middle.X+dir.X*l.Ahead, middle.Y+dir.Y*l.Ahead)
	ax, ay := dir.X*l.Length/2, dir.Y*l.Length/2
	cx, cy := -dir.Y*l.Width/2, dir.X*l.Width/2 // across, to the right of the way it is headed
	return [4]geom.Vec{
		geom.NewVec(middle.X-ax-cx, middle.Y-ay-cy),
		geom.NewVec(middle.X+ax-cx, middle.Y+ay-cy),
		geom.NewVec(middle.X-ax+cx, middle.Y-ay+cy),
		geom.NewVec(middle.X+ax+cx, middle.Y+ay+cy),
	}
}
