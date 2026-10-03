package cell

import (
	"image/color"

	"github.com/kjkrol/gram/render"
)

// Kind is a named terrain kind: whom it admits, what it does to movement and sight, and how
// it looks — its Color, or a sprite drawn for it (Kinds.Draw) or given by the game's atlas
// (board.Plugin.WithRenderer); how it looks in relief is a topography's (plugins/topography). A wall is
// Solid; water Allows Water; a hole Allows nobody and is not Solid; a forest Allows Land and has a
// Veil.
type Kind struct {
	Name Name // Named("grass")
	// Cost 1 is full speed and the cheapest step the planner counts on — a road; above 1 the cell
	// slows an entity and costs more to plan through — the ground off a road, say 2.5. Below 1
	// would be a boost past full speed, which the planner's estimate does not allow for.
	Cost float64
	// Allows is the domains that may stand here; the planner keeps the others out, and one that
	// ends up here anyway has fallen in — see unit.Standing.
	Allows Domain
	// Solid makes the cell solid ground: collision pushes out whoever Allows keeps out.
	Solid bool
	// Veil dims sight without blocking movement, 0 clear to 1 cutting it: a forest at 0.6 is looked
	// through at 0.4 of the reach, 1 cuts it. Apart from Solid.
	Veil float64
	// Veils is whom the Veil dims, as world.Layers: a forest veiling Land is looked over from Air.
	// Zero veils everyone.
	Veils Domain
	// Height is what stands on the cell (a wall, a forest) in a world with heights; a flat world refuses
	// it — see world.Config.Heights. The ground under it is the topography's. It is how far up a
	// veil holds sight back and, for a Solid kind, how high the wall collision stops entities at:
	// a Solid kind of no Height stands at every height.
	Height float64
	// Sway is how much what stands on the cell bends in the wind, 0 to 1: trees, reeds, corn — an
	// effect sets it when the wind blows.
	Sway float64
	// Graded ground is built up and cut into the slope — a road, a bridge — so the slope does not
	// slow whoever goes over it, nor count in a route: the kind's Cost is the whole price.
	Graded bool
	// Color is how the kind looks on a map drawn without an atlas of the game's: its cells filled
	// with it, its ways as bands of it. Zero is grey.
	Color    color.RGBA
	SpriteID render.SpriteID
	// Costs overrides Cost for entities moving in a domain — Costs[i] for the domain bit i, when
	// set; see Costing and CostFor.
	Costs [8]float64
}

// Admits reports whether an entity moving in d may stand on this kind.
func (k Kind) Admits(d Domain) bool { return k.Allows&d != 0 }

// Costing returns the kind with cost for the domains in d: elves through a forest, a witch over snow.
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

// Kinds is a board's registered set of Kinds, keyed by Name — reached through the board plugin
// (board.Plugin.Kinds), never built by the game. Names are strings here, as a Layout spells
// them.
type Kinds interface {
	// Create registers kinds, assigning each one's SpriteID by call order.
	Create(kinds ...Kind)
	// Get resolves name to the Kind registered under it.
	Get(name string) (Kind, bool)
	// All returns every registered Kind.
	All() []Kind
	// Draw has the kind named name drawn by draw on a map drawn without an atlas of the game's,
	// in place of its Color.
	Draw(name string, draw render.SpriteDrawer)
}
