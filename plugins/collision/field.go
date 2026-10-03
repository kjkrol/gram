package collision

import (
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// Field is the ground of a world laid out on a grid, for an entity on the layers given spanning
// band in height (Everywhere in a flat world). Solid calls visit with every solid box that box
// may overlap and whose own band meets band, its cell and its sides facing ground not solid for
// that entity, in the frame of the box asked about; false from visit ends the walk. Overhang is
// how much of box, as area, lies over ground that does not take the entity, heights aside. The
// board gives collision one (Plugin.WithField).
type Field interface {
	Solid(layers world.Layers, band Band, box geom.AABB, visit func(FieldBox) bool)
	Overhang(layers world.Layers, box geom.AABB) float64
}

// FieldBox is one solid box of a Field; see collide.FieldBox, which it is.
type FieldBox = collide.FieldBox
