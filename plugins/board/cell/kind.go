package cell

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/render"
)

// Kind is a named terrain kind: whom it admits and what it does to movement and sight. A wall is
// Solid; water Allows Water; a hole Allows nobody and is not Solid; a forest Allows Land and has
// a Veil.
type Kind struct {
	Name Name
	// Pace is the share of its top speed a unit keeps here: 1 full speed and nothing faster, 0.5
	// half; a kind admitting someone gives one above 0.
	Pace float64
	// Allows is the domains that may stand here; one that ends up here anyway has fallen in.
	Allows Domain
	// Solid makes the cell solid ground: collision pushes out whoever Allows keeps out.
	Solid bool
	// Veil dims sight without blocking movement, 0 clear to 1 cutting it.
	Veil float32
	// Veils is whom the Veil dims; zero veils everyone.
	Veils Domain
	// Height is how tall what stands on the cell is. A Solid or veiled kind of no Height stands at
	// every height.
	Height float32
	// Sway is how much what stands on the cell bends in the wind, 0 to 1.
	Sway float32
	// Graded ground is built into the slope, so the slope neither slows nor prices a step over it.
	Graded bool
	// Takes is which shade the cell takes and whether the lamps light it (render.Taking): zero
	// all, render.Shines none.
	Takes render.Taking
	// SpriteID is the kind's slot in the board's atlas, given as it is defined.
	SpriteID render.SpriteID
	// Paces overrides Pace for units moving in a domain, Paces[i] for the domain bit i; 0 leaves
	// Pace.
	Paces [8]float64
}

// Admits reports whether an entity moving in d may stand on this kind.
func (k Kind) Admits(d Domain) bool { return k.Allows&d != 0 }

// Paced returns the kind with share as the pace of the domains in d.
func (k Kind) Paced(d Domain, share float64) Kind {
	for i := range k.Paces {
		if d&(1<<i) != 0 {
			k.Paces[i] = share
		}
	}
	return k
}

// PaceFor is the share of its top speed a unit moving in d keeps here: the best among the
// domains this kind admits and paces, else Pace.
func (k Kind) PaceFor(d Domain) float64 {
	pace, paced := k.Pace, false
	for i := range k.Paces {
		if k.Paces[i] != 0 && d&k.Allows&(1<<i) != 0 && (!paced || k.Paces[i] > pace) {
			pace, paced = k.Paces[i], true
		}
	}
	return pace
}

// Of is one defined kind of cell, the handle a game builds on.
type Of struct{ k Kind }

// OfKind is k as the handle Named hands out.
func OfKind(k Kind) Of { return Of{k: k} }

// Kind is the kind as it was defined.
func (o Of) Kind() Kind { return o.k }

// SpriteID is the atlas slot the kind's cells are drawn from.
func (o Of) SpriteID() render.SpriteID { return o.k.SpriteID }

// Entry is a Layout entry laying this kind at c.
func (o Of) Entry(c ID) Entry { return Entry{Kind: o.k.Name.String(), Cell: c} }

// Kinds is a board's registered set of Kinds, keyed by name as a Layout spells them.
type Kinds interface {
	// Define registers k under name and has every cell laid as it play plays. A name defined
	// twice panics.
	Define(name string, k Kind, plays ...*ceremony.Part)
	// Named is the kind defined as name; an unknown name panics.
	Named(name string) Of
	// Get resolves name to the Kind registered under it.
	Get(name string) (Kind, bool)
	// All returns every registered Kind.
	All() []Kind
}
