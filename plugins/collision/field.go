package collision

import (
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// Field is the solid ground of a world laid out on a grid, for an entity on the layers given:
// Solid calls visit with every solid box that box may overlap, its cell and its sides that face
// open ground; visit returning false ends the walk. Boxes lie in the frame of the box asked
// about. The board gives collision one (Plugin.WithField); the engine pushes colliders out of it.
type Field interface {
	Solid(layers world.Layers, box geom.AABB, visit func(FieldBox) bool)
}

// FieldBox is one solid box of a Field; see collide.FieldBox, which it is.
type FieldBox = collide.FieldBox
