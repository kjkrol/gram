package response

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/world"
)

// Ground is how much of a box, as area, lies over ground that does not take an entity on layers.
type Ground interface {
	Overhang(layers world.Layers, box geom.AABB) float32
}

// Worse reports whether to lies further over ground that does not take an entity on layers than
// from.
func Worse(g Ground, layers world.Layers, from, to geom.AABB) bool {
	before := g.Overhang(layers, from)
	return g.Overhang(layers, to) > before+f32.Eps(before)
}

// Body is one side of a push apart: the planes it is on, its box, and whether nothing can move
// it.
type Body struct {
	Layers    world.Layers
	Box       geom.AABB
	Immovable bool
}

// Footing is the part of pen that moves a, given share by weight, the rest moving b against it:
// a side its push would put further over ground that does not take it holds where it is (holdA,
// holdB) and the other, movable, goes the whole way.
func Footing(g Ground, a, b Body, pen geom.Vec, share float32) (_ float32, holdA, holdB bool) {
	rest := 1 - share
	holdA = overhangs(g, a, geom.NewVec(pen.X*share, pen.Y*share))
	holdB = overhangs(g, b, geom.NewVec(-pen.X*rest, -pen.Y*rest))
	switch {
	case holdA == holdB || a.Immovable || b.Immovable:
		return share, holdA, holdB
	case holdA:
		return 0, true, false
	default:
		return 1, false, true
	}
}

func overhangs(g Ground, s Body, push geom.Vec) bool {
	if s.Immovable {
		return false
	}
	box := s.Box
	moved := geom.NewAABBAt(geom.NewVec(box.TopLeft.X+push.X, box.TopLeft.Y+push.Y), box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)
	return Worse(g, s.Layers, box, moved)
}
