package response

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// Ground is the ground under the colliders as the footing asks it: how much of box, as area, lies
// over ground that does not take an entity on layers. The collision's Field is one.
type Ground interface {
	Overhang(layers world.Layers, box geom.AABB) float64
}

// Worse reports whether to lies further over ground that does not take an entity on layers than
// from.
func Worse(g Ground, layers world.Layers, from, to geom.AABB) bool {
	return g.Overhang(layers, to) > g.Overhang(layers, from)+1e-9
}

// Body is one side of a push apart as the footing sees it: the planes it is on, its box, and
// whether nothing can move it.
type Body struct {
	Layers    world.Layers
	Box       geom.AABB
	Immovable bool
}

// Footing keeps two sides pushed apart by pen — half of it, a along it and b against it — on
// their ground: a side the push would put further over ground that does not take it holds where
// it is (holdA, holdB), and the other one, movable, goes the whole way. It gives the push to make.
func Footing(g Ground, a, b Body, pen geom.Vec) (push geom.Vec, holdA, holdB bool) {
	half := geom.NewVec(pen.X/2, pen.Y/2)
	holdA = overhangs(g, a, half)
	holdB = overhangs(g, b, geom.NewVec(-half.X, -half.Y))
	if holdA != holdB && !a.Immovable && !b.Immovable {
		return geom.NewVec(2*pen.X, 2*pen.Y), holdA, holdB // the half the one held does not take, the other does
	}
	return pen, holdA, holdB
}

// overhangs reports whether s pushed by push would lie further over ground that does not take it.
func overhangs(g Ground, s Body, push geom.Vec) bool {
	if s.Immovable {
		return false
	}
	box := s.Box
	moved := geom.NewAABBAt(geom.NewVec(box.TopLeft.X+push.X, box.TopLeft.Y+push.Y), box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)
	return Worse(g, s.Layers, box, moved)
}
