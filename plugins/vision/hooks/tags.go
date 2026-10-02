package hooks

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/world"
)

// Family is the tag family of the ready-made vision rules.
type Family struct{}

// Tags is the ready-made tags: who steers clear, who is fled from on sight, who hunts and who
// is hunted. Define them once with DefineTags and give them to kinds with comp.Tagged.
type Tags struct {
	Skittish, Threat, Predator, Prey tag.Tag[Family]
}

// DefineTags registers the ready-made tags with the world's Kinds.
func DefineTags(reg *world.Kinds) Tags {
	return Tags{
		Skittish: reg.DefineTag[Family]("vision.skittish"),
		Threat:   reg.DefineTag[Family]("vision.threat"),
		Predator: reg.DefineTag[Family]("vision.predator"),
		Prey:     reg.DefineTag[Family]("vision.prey"),
	}
}
