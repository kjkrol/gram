package collision

import (
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// Field is the solid ground of a world, as the board gives it (Plugin.WithField). Solid and
// Walls visit every solid box and wall an entity on layers spanning band may overlap within box
// (false from visit ends the walk); Overhang is how much of box, as area, lies over ground that
// does not take the entity.
type Field interface {
	Solid(layers world.Layers, band Band, box geom.AABB, visit func(FieldBox) bool)
	Walls(layers world.Layers, band Band, box geom.AABB, visit func(collide.FieldWall) bool)
	Overhang(layers world.Layers, box geom.AABB) float64
}

// FieldBox is one solid box of a Field.
type FieldBox = collide.FieldBox
