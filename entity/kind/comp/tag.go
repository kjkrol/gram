package comp

import (
	"github.com/kjkrol/gram/entity/tag"
)

// Tagged is a Spec component giving every entity of the kind these tags of one family; a Spec
// names each family once. The tags come from world.Kinds.DefineTag.
func Tagged[F any](tags ...tag.Tag[F]) Template[tag.Tags[F]] {
	return Const(tag.Tags[F](0).With(tags...))
}

// Marks is a Spec component giving every entity of the kind the markers of family F for good, all
// off: states a plugin switches on and off by a bit, the entity never gaining or losing a
// component for them. A plugin gives its family to every unit through the world's roster
// (Template.Default).
func Marks[F any]() Template[tag.Tags[F]] { return Const(tag.Tags[F](0)) }
