package cell

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule"
)

// Entry sets Cell to the kind named Kind (none: the Layout's Default) and gives it the Name it
// bears, the Group it is in and the Roles this one cell plays beyond its kind's.
type Entry struct {
	Kind  string
	Cell  ID
	Name  string
	Group string
	Roles tag.Tags[rule.Roles]
}

// Named is the entry with the name this one cell bears.
func (e Entry) Named(name string) Entry { e.Name = name; return e }

// InGroup is the entry in group.
func (e Entry) InGroup(group string) Entry { e.Group = group; return e }

// Plays is the entry playing roles beyond its kind's.
func (e Entry) Plays(roles ...*rule.Part) Entry {
	for _, r := range roles {
		e.Roles = e.Roles.With(r.Tag())
	}
	return e
}

// WayEntry lays a Way of the kind named Kind across Cell.
type WayEntry struct {
	Kind  string
	Cell  ID
	Width float32
	Links Links
	Fade  float32
	Mix   float32
}
