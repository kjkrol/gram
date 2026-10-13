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
	// Cost 1 is full speed and the cheapest step the planner counts on; above 1 slows and costs
	// more to plan through. Below 1 breaks the planner's estimate.
	Cost float64
	// Allows is the domains that may stand here; one that ends up here anyway has fallen in.
	Allows Domain
	// Solid makes the cell solid ground: collision pushes out whoever Allows keeps out.
	Solid bool
	// Veil dims sight without blocking movement, 0 clear to 1 cutting it.
	Veil float64
	// Veils is whom the Veil dims; zero veils everyone.
	Veils Domain
	// Height is what stands on the cell in a world with heights; a flat world refuses it. A Solid
	// kind of no Height stands at every height.
	Height float64
	// Sway is how much what stands on the cell bends in the wind, 0 to 1.
	Sway float64
	// Graded ground is built into the slope, so the slope neither slows nor prices a step over it.
	Graded bool
	// SpriteID is the kind's slot in the board's atlas, given as it is defined.
	SpriteID render.SpriteID
	// Costs overrides Cost for entities moving in a domain, Costs[i] for the domain bit i.
	Costs [8]float64
}

// Admits reports whether an entity moving in d may stand on this kind.
func (k Kind) Admits(d Domain) bool { return k.Allows&d != 0 }

// Costing returns the kind with cost for the domains in d.
func (k Kind) Costing(d Domain, cost float64) Kind {
	for i := range k.Costs {
		if d&(1<<i) != 0 {
			k.Costs[i] = cost
		}
	}
	return k
}

// CostFor is what an entity moving in d pays here: the cheapest of its domains this kind admits
// and prices, else Cost.
func (k Kind) CostFor(d Domain) float64 {
	cost, priced := k.Cost, false
	for i := range k.Costs {
		if k.Costs[i] != 0 && d&k.Allows&(1<<i) != 0 && (!priced || k.Costs[i] < cost) {
			cost, priced = k.Costs[i], true
		}
	}
	return cost
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
