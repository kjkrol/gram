package unit

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
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

// Placed: a rule's Here acts on the cells under the entity's box, its Around on the rings round
// them too.
func (Standing) Placed() {}

// Fallen reports whether the entity stands where its domain may not be: in a hole, in water on
// foot; never while it is in the air.
func (s Standing) Fallen() bool { return !s.Airborne && !s.Kind.Admits(s.Domain) }

// On is the condition of a unit standing on a cell of the kind k, for a rule's When.
func On(k cell.Kind) func(Standing) bool { return func(s Standing) bool { return s.Kind == k } }

// Over is the condition of a unit standing on a cell under the effect e, for a rule's When: the
// ground's state, whatever its kind.
func Over(e effect.Effect) func(Standing) bool {
	mark := e.Mark()
	return func(s Standing) bool { return s.States.Has(mark) }
}
