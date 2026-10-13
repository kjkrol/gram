package unit

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Standing is where an entity on the board stands this tick: the cell under its centre, its kind
// and the effects on it, the entity's box and the domains it moves in (Land without a Mover).
type Standing struct {
	ID       uid.UID64
	Cell     cell.ID
	Kind     cell.Kind
	States   tag.Tags[effect.States] // the effects on the cell now: frozen over, snowed under
	Box      geom.AABB
	Domain   cell.Domain
	Airborne bool // in the air over the cell, not on it
}

// Who is the entity standing: whose moment it is, for a rule.
func (s Standing) Who() uid.UID64 { return s.ID }

// Placed: a rule's Around turns to the rings of cells round those under the entity's box.
func (Standing) Placed() {}

// Fallen is the word of a unit standing where its domain may not be: in a hole, in water on
// foot; never while it is in the air.
func Fallen() cond.Word[Standing] { return cond.NewWord("fallen", Standing.fallen) }

func (s Standing) fallen() bool { return !s.Airborne && !s.Kind.Admits(s.Domain) }

// OnKind is the word of a unit standing on a cell of the kind k.
func OnKind(k cell.Kind) cond.Word[Standing] {
	return cond.NewWord("on a kind", func(s Standing) bool { return s.Kind == k })
}

// StandingOn is the word of a unit standing on a cell under the effect e: the ground's state,
// whatever its kind.
func StandingOn(e effect.Effect) cond.Word[Standing] {
	mark := e.Mark()
	return cond.NewWord("standing on", func(s Standing) bool { return s.States.Has(mark) })
}
