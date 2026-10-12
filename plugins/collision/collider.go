package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// Collider makes an entity take part in collision detection and holds what it struck the tick
// before. Two colliders touch only where their world.Layers meet.
type Collider struct {
	Struck      [MaxContacts]Contact
	StruckCount uint8
}

// Contacts is what this entity struck the tick before, in the order confirmed.
func (c *Collider) Contacts() []Contact { return c.Struck[:c.StruckCount] }

func (c *Collider) addContact(other uid.UID64, impact float64, normal geom.Vec, along float64, sensed bool) {
	c.add(Contact{Other: other, Impact: impact, Normal: normal, Along: along, Sensed: sensed})
}

func (c *Collider) add(ct Contact) {
	if c.StruckCount < MaxContacts {
		c.Struck[c.StruckCount] = ct
		c.StruckCount++
	}
}

func (c *Collider) clearContacts() { c.StruckCount = 0 }
