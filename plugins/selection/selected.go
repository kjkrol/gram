package selection

import (
	"github.com/kjkrol/gram/entity/tag"
)

// Family is selection's tag family: Selectable, Selected and Followed live in it.
type Family struct{}

// Tags is selection's tags: Selectable marks an entity the player may select, Selected one
// the player has, Followed the one the camera follows. A kind gives Selectable with comp.Tagged;
// the plugin flips Selected and Followed.
type Tags struct {
	Selectable, Selected, Followed tag.Tag[Family]
}
