package board

import (
	"fmt"

	"github.com/kjkrol/gram/render"
)

// Terrain reports one cell's terrain kind, independent of the Grid's topology.
type Terrain interface {
	Kind(c CellID) CellKind
}

// CellKind is a named terrain kind: whom it admits, what it does to movement and sight, and the
// sprite drawn for it. A wall is Solid; water Allows Water; a hole Allows nobody and is not
// Solid; a forest Allows Land and has a Veil.
type CellKind struct {
	Name Name // Named("grass")
	// Cost 1 is full speed and the baseline path weight; above 1 the cell slows an entity and costs
	// more to plan through, below 1 is a boost — a game's choice, still capped by the move's step.
	Cost float64
	// Allows is the domains that may stand here; the planner keeps the others out, and one that
	// ends up here anyway has fallen in — see Standing.
	Allows Domain
	// Solid makes the cell solid ground: collision pushes out whoever Allows keeps out.
	Solid bool
	// Veil dims sight without blocking movement, 0 clear to 1 cutting it: a forest at 0.6 is looked
	// through at 0.4 of the reach, 1 cuts it. Apart from Solid.
	Veil float64
	// Veils is whom the Veil dims, as world.Layers: a forest veiling Land is looked over from Air.
	// Zero veils everyone.
	Veils Domain
	// Height is what stands on the cell (a wall, a forest) in a Quasi3D world; a flat world refuses
	// it — see world.Config.Quasi3D. The ground under it is the cell's Relief.
	Height float64
	// Shine is how much of the sun the cell's surface throws back at whoever looks at it, 0 to 1:
	// water, ice, wet rock glint where the sun and the eye meet over its ripples.
	Shine float64
	// Flow is how fast the water on the cell runs down its slope: world units a second where it
	// falls 1 in 1, by the square root of the slope; 0 is still water, a sea or a lake. Drawn on a
	// square grid, whose cells slope; a hex cell is level and its water still.
	Flow float64
	// Sway is how much what stands on the cell bends in the wind, 0 to 1: trees, reeds, corn — an
	// effect sets it when the wind blows.
	Sway     float64
	SpriteID render.SpriteID
	// Costs overrides Cost for entities moving in a domain — Costs[i] for the domain bit i, when
	// set; see Costing and CostFor.
	Costs [8]float64
}

// Admits reports whether an entity moving in d may stand on this kind.
func (k CellKind) Admits(d Domain) bool { return k.Allows&d != 0 }

// Costing returns the kind with cost for the domains in d: elves through a forest, a witch over snow.
func (k CellKind) Costing(d Domain, cost float64) CellKind {
	for i := range k.Costs {
		if d&(1<<i) != 0 {
			k.Costs[i] = cost
		}
	}
	return k
}

// CostFor is what an entity moving in d pays here: the cheapest of its domains this kind admits
// and prices, else Cost.
func (k CellKind) CostFor(d Domain) float64 {
	cost, priced := k.Cost, false
	for i := range k.Costs {
		if k.Costs[i] != 0 && d&k.Allows&(1<<i) != 0 && (!priced || k.Costs[i] < cost) {
			cost, priced = k.Costs[i], true
		}
	}
	return cost
}

// CellKindDict is a Plugin's registered set of CellKinds, keyed by Name —
// reached via Plugin.CellKindDict, never built directly by the game. Names are strings here, as a
// Layout spells them.
type CellKindDict interface {
	// Create registers kinds, assigning each one's SpriteID by call order.
	Create(kinds ...CellKind)
	// Get resolves name to the CellKind registered under it.
	Get(name string) (CellKind, bool)
	// All returns every registered CellKind.
	All() []CellKind
}

type cellKindDict struct {
	entries map[Name]CellKind
	next    render.SpriteID
	quasi3D bool
}

func newCellKindDict(quasi3D bool) *cellKindDict {
	return &cellKindDict{entries: make(map[Name]CellKind), quasi3D: quasi3D}
}

func (d *cellKindDict) Create(kinds ...CellKind) {
	for _, k := range kinds {
		if !d.quasi3D && k.Height != 0 {
			panic(fmt.Sprintf("board: kind %q has a Height in a flat world; set world.Config.Quasi3D", k.Name.String()))
		}
		k.SpriteID = d.next
		d.next++
		d.entries[k.Name] = k
	}
}

func (d *cellKindDict) Get(name string) (CellKind, bool) {
	n, ok := nameOf(name)
	if !ok {
		return CellKind{}, false
	}
	k, ok := d.entries[n]
	return k, ok
}

func (d *cellKindDict) All() []CellKind {
	all := make([]CellKind, 0, len(d.entries))
	for _, k := range d.entries {
		all = append(all, k)
	}
	return all
}

// TerrainMap is a Terrain backed by a plain map: a Board's seed until the ECS is set up, and its
// whole terrain on a board no ECS runs. Change the terrain through the Board.
type TerrainMap struct {
	Cells   map[CellID]CellKind
	Default CellKind

	version uint64
}

var _ Terrain = (*TerrainMap)(nil)

func NewTerrainMap() *TerrainMap {
	return &TerrainMap{Cells: make(map[CellID]CellKind)}
}

func (t *TerrainMap) Kind(c CellID) CellKind {
	if kind, ok := t.Cells[c]; ok {
		return kind
	}
	return t.Default
}

// Set assigns c's terrain kind, taking effect immediately.
func (t *TerrainMap) Set(c CellID, kind CellKind) {
	if t.Cells[c] == kind {
		return
	}
	t.Cells[c] = kind
	t.version++
}

// SetMany assigns kind to every cell in cells in one call, instead of looping Set per cell.
func (t *TerrainMap) SetMany(cells []CellID, kind CellKind) {
	changed := false
	for _, c := range cells {
		if t.Cells[c] != kind {
			t.Cells[c] = kind
			changed = true
		}
	}
	if changed {
		t.version++
	}
}

// SetAll resets every cell's terrain kind to kind, discarding any prior Set/SetMany overrides.
func (t *TerrainMap) SetAll(kind CellKind) {
	clear(t.Cells)
	t.Default = kind
	t.version++
}

// Version counts the changes made through Set, SetMany and SetAll — a write that changes nothing
// does not count; a load starts it over.
func (t *TerrainMap) Version() uint64 { return t.version }
