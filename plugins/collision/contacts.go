package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// MaxContacts caps how many confirmed contacts one entity records in a tick;
// extras are still separated, bounced and reported to pair rules.
const MaxContacts = 8

// Contact is one confirmed contact: who it was against, the impulse exchanged
// (zero when only sensed or already separating), and the way this entity leaves the other.
// Against the world's solid ground Terrain is set, Cell names the cell and Other is zero.
type Contact struct {
	Other   uid.UID64
	Impact  float64
	Normal  geom.Vec
	Terrain bool
	Cell    uint64
}
