package cell

import (
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule"
)

// Terrain reports one cell's terrain kind, independent of the grid's topology.
type Terrain interface {
	Kind(c ID) Kind
}

// TerrainMap is a Terrain backed by plain maps: a board's seed until the ECS is set up, and its
// whole terrain on a board no ECS runs. Change the terrain through the board.
type TerrainMap struct {
	Cells   map[ID]Kind
	Default Kind
	// Ways is what runs across the cells over their kinds, Crossings what crosses over the ways;
	// Kind leaves them out — the board lays them over.
	Ways      map[ID]Way
	Crossings map[ID]Crossing
	// Roles are the roles the cells play and Labels what they are called — a name, a group — for
	// good, the Layout's.
	Roles  map[ID]tag.Tags[rule.Roles]
	Labels map[ID]entity.Label

	version uint64
}

var _ Terrain = (*TerrainMap)(nil)

func NewTerrainMap() *TerrainMap {
	return &TerrainMap{Cells: make(map[ID]Kind), Ways: make(map[ID]Way), Crossings: make(map[ID]Crossing)}
}

func (t *TerrainMap) Kind(c ID) Kind {
	if kind, ok := t.Cells[c]; ok {
		return kind
	}
	return t.Default
}

// Set assigns c's terrain kind, taking effect immediately.
func (t *TerrainMap) Set(c ID, kind Kind) {
	if t.Cells[c] == kind {
		return
	}
	t.Cells[c] = kind
	t.version++
}

// SetWay lays w across c, the zero Way taking what ran there away.
func (t *TerrainMap) SetWay(c ID, w Way) {
	if t.Ways[c] == w {
		return
	}
	if !w.Runs() {
		delete(t.Ways, c)
	} else {
		if t.Ways == nil {
			t.Ways = make(map[ID]Way)
		}
		t.Ways[c] = w
	}
	t.version++
}

// SetCrossing lays x across c over its way, the zero Crossing taking what crossed there away.
func (t *TerrainMap) SetCrossing(c ID, x Crossing) {
	if t.Crossings[c] == x {
		return
	}
	if !x.Runs() {
		delete(t.Crossings, c)
	} else {
		if t.Crossings == nil {
			t.Crossings = make(map[ID]Crossing)
		}
		t.Crossings[c] = x
	}
	t.version++
}

// Cast has c play roles besides those it plays; the terrain's Version stays as it was.
func (t *TerrainMap) Cast(c ID, roles tag.Tags[rule.Roles]) {
	if t.Roles == nil {
		t.Roles = make(map[ID]tag.Tags[rule.Roles])
	}
	t.Roles[c] |= roles
}

// Label calls c as l says; the terrain's Version stays as it was.
func (t *TerrainMap) Label(c ID, l entity.Label) {
	if t.Labels == nil {
		t.Labels = make(map[ID]entity.Label)
	}
	t.Labels[c] = l
}

// SetAll resets every cell's terrain kind to kind, discarding any prior Set.
func (t *TerrainMap) SetAll(kind Kind) {
	clear(t.Cells)
	t.Default = kind
	t.version++
}

// Version counts the changes made through Set, SetAll, SetWay and SetCrossing — a write that
// changes nothing does not count; a load starts it over.
func (t *TerrainMap) Version() uint64 { return t.version }
