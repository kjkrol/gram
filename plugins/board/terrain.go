package board

import (
	"fmt"
	"image/color"

	"github.com/kjkrol/gram/render"
)

// Terrain reports one cell's terrain kind, independent of the Grid's topology.
type Terrain interface {
	Kind(c CellID) CellKind
}

// CellKind is a named terrain kind: whom it admits, what it does to movement and sight, and how
// it looks — its Color, or a sprite drawn for it (CellKindDict.Draw) or given by the game's atlas
// (Plugin.WithRenderer); how it looks in relief is a topography's (plugins/topography). A wall is
// Solid; water Allows Water; a hole Allows nobody and is not Solid; a forest Allows Land and has a
// Veil.
type CellKind struct {
	Name Name // Named("grass")
	// Cost 1 is full speed and the cheapest step the planner counts on — a road; above 1 the cell
	// slows an entity and costs more to plan through — the ground off a road, say 2.5. Below 1
	// would be a boost past full speed, which the planner's estimate does not allow for.
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
	// Height is what stands on the cell (a wall, a forest) in a world with heights; a flat world refuses
	// it — see world.Config.Heights. The ground under it is the topography's.
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
	// Draw has the kind named name drawn by draw on a map drawn without an atlas of the game's,
	// in place of its Color.
	Draw(name string, draw render.SpriteDrawer)
}

type cellKindDict struct {
	entries map[Name]CellKind
	drawers map[Name]render.SpriteDrawer
	next    render.SpriteID
	heights bool
}

func newCellKindDict(heights bool) *cellKindDict {
	return &cellKindDict{entries: make(map[Name]CellKind), drawers: make(map[Name]render.SpriteDrawer), heights: heights}
}

func (d *cellKindDict) Draw(name string, draw render.SpriteDrawer) { d.drawers[Named(name)] = draw }

func (d *cellKindDict) Create(kinds ...CellKind) {
	for _, k := range kinds {
		if !d.heights && k.Height != 0 {
			panic(fmt.Sprintf("board: kind %q has a Height in a flat world; set world.Config.Heights", k.Name.String()))
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
	// Ways is what runs across the cells over their kinds, Crossings what crosses over the ways;
	// Kind leaves them out — Board.Kind lays them over.
	Ways      map[CellID]Way
	Crossings map[CellID]Crossing

	version uint64
}

var _ Terrain = (*TerrainMap)(nil)

func NewTerrainMap() *TerrainMap {
	return &TerrainMap{Cells: make(map[CellID]CellKind), Ways: make(map[CellID]Way), Crossings: make(map[CellID]Crossing)}
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

// SetWay lays w across c, the zero Way taking what ran there away.
func (t *TerrainMap) SetWay(c CellID, w Way) {
	if t.Ways[c] == w {
		return
	}
	if !w.Runs() {
		delete(t.Ways, c)
	} else {
		if t.Ways == nil {
			t.Ways = make(map[CellID]Way)
		}
		t.Ways[c] = w
	}
	t.version++
}

// SetCrossing lays x across c over its way, the zero Crossing taking what crossed there away.
func (t *TerrainMap) SetCrossing(c CellID, x Crossing) {
	if t.Crossings[c] == x {
		return
	}
	if !x.Runs() {
		delete(t.Crossings, c)
	} else {
		if t.Crossings == nil {
			t.Crossings = make(map[CellID]Crossing)
		}
		t.Crossings[c] = x
	}
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

// Version counts the changes made through Set, SetMany, SetAll, SetWay and SetCrossing — a write that changes
// nothing does not count; a load starts it over.
func (t *TerrainMap) Version() uint64 { return t.version }
