package cell

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule"
)

// Entry sets Cell to the kind named Kind — none, the board's Layout's Default kept — and gives it,
// for good, the Name it bears, its alone, the Group it is in with others — what commands find
// it by (entity.Named, entity.Group) — and the Roles this one cell plays beyond its kind's: the
// cells of the lake, not every water. Build one on the kind's handle (Of.Entry) and chain Named, InGroup
// and Plays, or write it out.
type Entry struct {
	Kind  string
	Cell  ID
	Name  string
	Group string
	Roles tag.Tags[rule.Roles]
}

// Named is the entry with the name this one cell bears, its alone.
func (e Entry) Named(name string) Entry { e.Name = name; return e }

// InGroup is the entry in group, with the others called so.
func (e Entry) InGroup(group string) Entry { e.Group = group; return e }

// Plays is the entry playing roles, for good, beyond its kind's: the rules they obey fire for
// this cell.
func (e Entry) Plays(roles ...*rule.Part) Entry {
	for _, r := range roles {
		e.Roles = e.Roles.With(r.Tag())
	}
	return e
}

// WayEntry lays a Way of the kind named Kind across Cell, Width wide, running on as Links says,
// faded out as far as Fade, its look turned as far as Mix.
type WayEntry struct {
	Kind  string
	Cell  ID
	Width float32
	Links Links
	Fade  float32
	Mix   float32
}
