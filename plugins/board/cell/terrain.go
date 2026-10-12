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

// TerrainMap is a Terrain backed by plain maps: a board's seed, and its whole terrain on a board
// no ECS runs.
type TerrainMap struct {
	Cells   map[ID]Kind
	Default Kind
	// Ways is what runs across the cells over their kinds; Kind leaves them out.
	Ways map[ID]Way
	// Roles are the roles the cells play and Labels what they are called.
	Roles  map[ID]tag.Tags[rule.Roles]
	Labels map[ID]entity.Label

	version uint64
}

var _ Terrain = (*TerrainMap)(nil)

func NewTerrainMap() *TerrainMap {
	return &TerrainMap{Cells: make(map[ID]Kind), Ways: make(map[ID]Way)}
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

// Cast has c play roles, none for 0, in place of those it played; Version stays as it was.
func (t *TerrainMap) Cast(c ID, roles tag.Tags[rule.Roles]) {
	if roles == 0 {
		delete(t.Roles, c)
		return
	}
	if t.Roles == nil {
		t.Roles = make(map[ID]tag.Tags[rule.Roles])
	}
	t.Roles[c] = roles
}

// Label calls c as l says; Version stays as it was.
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

// Version counts the changes made through Set, SetAll and SetWay; a write that changes nothing
// does not count.
func (t *TerrainMap) Version() uint64 { return t.version }
