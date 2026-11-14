package entity

import "github.com/kjkrol/aabbworld/geom"

// Stand is the rectangle a thing standing turned covers on the ground: Length along its heading
// (Velocity.Dir) and Width across, round the middle of its box. Zero, it stands over its box.
type Stand struct{ Length, Width float32 }

// Corners are the corners of the rectangle, the box's middle at middle and headed dir, in the
// order of Lie's.
func (s Stand) Corners(middle, dir geom.Vec) [4]geom.Vec {
	return Lie{Length: s.Length, Width: s.Width}.Corners(middle, dir)
}
