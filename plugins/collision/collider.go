package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// Collider is what makes an entity take part in collision detection — carrying it is
// all it takes — and holds what the narrow phase confirmed the entity struck. Two colliders touch
// only where their world.Layers meet, so a flyer and a walker on different planes pass through
// each other.
type Collider struct {
	Struck      [MaxContacts]Contact
	StruckCount uint8
}

// Contacts is what this entity struck the tick before, in the order confirmed.
func (c *Collider) Contacts() []Contact { return c.Struck[:c.StruckCount] }

// addContact records one confirmed contact with other, up to MaxContacts.
func (c *Collider) addContact(other uid.UID64, impact float64, normal geom.Vec, along float64, sensed bool) {
	c.add(Contact{Other: other, Impact: impact, Normal: normal, Along: along, Sensed: sensed})
}

func (c *Collider) add(ct Contact) {
	if c.StruckCount < MaxContacts {
		c.Struck[c.StruckCount] = ct
		c.StruckCount++
	}
}

// clearContacts drops the previous tick's contacts.
func (c *Collider) clearContacts() { c.StruckCount = 0 }
