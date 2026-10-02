package collision

import (
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// Field is the ground of a world laid out on a grid, for an entity on the layers given. Solid calls
// visit with every solid box that box may overlap, its cell and its sides that face open ground;
// visit returning false ends the walk; boxes lie in the frame of the box asked about. Overhang is
// how much of box, as area, lies over ground that does not take the entity — water to a walker, a
// hole. The board gives collision one (Plugin.WithField): the engine pushes colliders out of the
// solid ground, and a push apart never makes a collider overhang more.
type Field interface {
	Solid(layers world.Layers, box geom.AABB, visit func(FieldBox) bool)
	Overhang(layers world.Layers, box geom.AABB) float64
}

// FieldBox is one solid box of a Field; see collide.FieldBox, which it is.
type FieldBox = collide.FieldBox
