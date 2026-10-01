package board

import (
	"fmt"
	"github.com/kjkrol/gram/plugins/board/cell"

	"github.com/kjkrol/gram/render"
)

// Terrain reports one cell's terrain kind, independent of the Grid's topology.
type Terrain interface {
	Kind(c cell.ID) cell.Kind
}

type cellKindDict struct {
	entries map[cell.Name]cell.Kind
	drawers map[cell.Name]render.SpriteDrawer
	next    render.SpriteID
	heights bool
}

func newCellKindDict(heights bool) *cellKindDict {
	return &cellKindDict{entries: make(map[cell.Name]cell.Kind), drawers: make(map[cell.Name]render.SpriteDrawer), heights: heights}
}

func (d *cellKindDict) Draw(name string, draw render.SpriteDrawer) {
	d.drawers[cell.Named(name)] = draw
}

func (d *cellKindDict) Create(kinds ...cell.Kind) {
	for _, k := range kinds {
		if !d.heights && k.Height != 0 {
			panic(fmt.Sprintf("board: kind %q has a Height in a flat world; set world.Config.Heights", k.Name.String()))
		}
		k.SpriteID = d.next
		d.next++
		d.entries[k.Name] = k
	}
}

func (d *cellKindDict) Get(name string) (cell.Kind, bool) {
	if len(name) > cell.MaxNameLen {
		return cell.Kind{}, false
	}
	k, ok := d.entries[cell.Named(name)]
	return k, ok
}

func (d *cellKindDict) All() []cell.Kind {
	all := make([]cell.Kind, 0, len(d.entries))
	for _, k := range d.entries {
		all = append(all, k)
	}
	return all
}

// TerrainMap is a Terrain backed by a plain map: a Board's seed until the ECS is set up, and its
// whole terrain on a board no ECS runs. Change the terrain through the Board.
type TerrainMap struct {
	Cells   map[cell.ID]cell.Kind
	Default cell.Kind
	// Ways is what runs across the cells over their kinds, Crossings what crosses over the ways;
	// Kind leaves them out — Board.Kind lays them over.
	Ways      map[cell.ID]cell.Way
	Crossings map[cell.ID]cell.Crossing

	version uint64
}

var _ Terrain = (*TerrainMap)(nil)

func NewTerrainMap() *TerrainMap {
	return &TerrainMap{Cells: make(map[cell.ID]cell.Kind), Ways: make(map[cell.ID]cell.Way), Crossings: make(map[cell.ID]cell.Crossing)}
}

func (t *TerrainMap) Kind(c cell.ID) cell.Kind {
	if kind, ok := t.Cells[c]; ok {
		return kind
	}
	return t.Default
}

// Set assigns c's terrain kind, taking effect immediately.
func (t *TerrainMap) Set(c cell.ID, kind cell.Kind) {
	if t.Cells[c] == kind {
		return
	}
	t.Cells[c] = kind
	t.version++
}

// SetWay lays w across c, the zero Way taking what ran there away.
func (t *TerrainMap) SetWay(c cell.ID, w cell.Way) {
	if t.Ways[c] == w {
		return
	}
	if !w.Runs() {
		delete(t.Ways, c)
	} else {
		if t.Ways == nil {
			t.Ways = make(map[cell.ID]cell.Way)
		}
		t.Ways[c] = w
	}
	t.version++
}

// SetCrossing lays x across c over its way, the zero Crossing taking what crossed there away.
func (t *TerrainMap) SetCrossing(c cell.ID, x cell.Crossing) {
	if t.Crossings[c] == x {
		return
	}
	if !x.Runs() {
		delete(t.Crossings, c)
	} else {
		if t.Crossings == nil {
			t.Crossings = make(map[cell.ID]cell.Crossing)
		}
		t.Crossings[c] = x
	}
	t.version++
}

// SetMany assigns kind to every cell in cells in one call, instead of looping Set per cell.
func (t *TerrainMap) SetMany(cells []cell.ID, kind cell.Kind) {
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
func (t *TerrainMap) SetAll(kind cell.Kind) {
	clear(t.Cells)
	t.Default = kind
	t.version++
}

// Version counts the changes made through Set, SetMany, SetAll, SetWay and SetCrossing — a write that changes
// nothing does not count; a load starts it over.
func (t *TerrainMap) Version() uint64 { return t.version }
