package selection

import (
	"github.com/kjkrol/gram/entity/tag"
)

// Family is selection's tag family: Selectable and Selected live in it.
type Family struct{}

// Tags is selection's tags: Selectable marks an entity the player may select, Selected one
// the player has. A kind gives Selectable with comp.Tagged; the plugin flips Selected.
type Tags struct {
	Selectable, Selected tag.Tag[Family]
}
