package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// MaxContacts caps how many contacts one entity records in a tick; extras are still separated,
// bounced and reported to rules.
const MaxContacts = 8

// Contact is one confirmed contact: who it was against (the solid ground with Terrain set and
// its Cell), the impulse exchanged and the way this entity leaves the other. Along is where
// along a swept entity's step it lies, 0 to 1 (1 with none swept); Sensed that it was only
// detected; Resting that the two, parted before, still touch: no push, no strike.
type Contact struct {
	Other   uid.UID64
	Impact  float64
	Normal  geom.Vec
	Terrain bool
	Cell    uint64
	Along   float64
	Sensed  bool
	Resting bool
}
