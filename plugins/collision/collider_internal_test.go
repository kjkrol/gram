package collision

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

func TestCollider_AddContact_CapsAtMaxContacts(t *testing.T) {
	var c Collider
	for i := range MaxContacts + 5 {
		c.addContact(uid.UID64(i), float64(i), geom.NewVec(1, 0), 1, false)
	}

	got := c.Contacts()
	if len(got) != MaxContacts {
		t.Fatalf("%d contacts recorded, want %d (extras past the cap dropped)", len(got), MaxContacts)
	}
	for i, contact := range got {
		if contact.Other != uid.UID64(i) {
			t.Errorf("contact %d is against %v, want %v — the first ones are the ones kept", i, contact.Other, uid.UID64(i))
		}
	}
}

func TestCollider_ClearContacts(t *testing.T) {
	var c Collider
	c.addContact(uid.UID64(1), 1, geom.NewVec(1, 0), 1, false)
	c.addContact(uid.UID64(2), 1, geom.NewVec(1, 0), 1, false)

	c.clearContacts()

	if got := c.Contacts(); len(got) != 0 {
		t.Errorf("%d contacts left after clearContacts, want none", len(got))
	}
}
