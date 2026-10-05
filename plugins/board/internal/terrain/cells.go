package terrain

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/grids"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Cells is a board's cells' terrain: its seed until the ECS is set up, then the cells' entities,
// with the counts of changes to them. Not safe for concurrent use.
type Cells struct {
	grid     grid.Grid
	square   uint64 // a square grid's cell count, its ids counting row by row; 0 for any other
	seed     *cell.TerrainMap
	store    *store // the cells' entities; nil before Setup
	version  uint64
	stamps   []uint64 // by ordinal: the count of changes when each cell last changed
	changes  uint64   // how many cells have changed, one at a time
	everyone uint64   // the count when every cell last changed at once
}

// New is the terrain of g's cells, every one of the zero kind until it is set.
func New(g grid.Grid) *Cells {
	t := &Cells{grid: g, seed: cell.NewTerrainMap(), stamps: make([]uint64, g.CellCount())}
	if sq, ok := g.(*grids.Square); ok {
		t.square = uint64(sq.Width) * uint64(sq.Height)
	}
	return t
}

// System makes an entity for every cell at Setup — each carrying template's defaults besides, a
// Load reading its cell.ID — or finds those a save brought back, and counts the changes effects
// make to them.
func (t *Cells) System(template *kind.Template) goke.System {
	return &entitySystem{cells: t, template: template}
}

// Made reports whether the cells are entities yet.
func (t *Cells) Made() bool { return t.store != nil }

// Ordinal is the grid's, straight from the id on a square grid.
func (t *Cells) Ordinal(c cell.ID) (int, bool) {
	if t.square != 0 {
		return int(c), uint64(c) < t.square
	}
	return t.grid.Ordinal(c)
}

// Entity is c's entity; false off the board or before the cells are made.
func (t *Cells) Entity(c cell.ID) (uid.UID64, bool) {
	i, ok := t.Ordinal(c)
	if !ok || t.store == nil {
		return 0, false
	}
	return t.store.ids[i], true
}

// Kind is c's kind as whoever crosses it meets it: the ground's under its Way and its Crossing;
// off the board, the zero kind.
func (t *Cells) Kind(c cell.ID) cell.Kind {
	if t.store == nil {
		return t.seed.Crossings[c].Over(t.seed.Ways[c].Over(t.seed.Kind(c)))
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Kind{}
	}
	return t.store.crossingOf(i).Over(t.store.wayOf(i).Over(t.store.groundOf(i).Kind))
}

// Bare is c's ground kind, bare of what runs across it; off the board, the zero kind.
func (t *Cells) Bare(c cell.ID) cell.Kind {
	if k := t.KindOf(c); k != nil {
		return *k
	}
	return cell.Kind{}
}

// KindOf is c's ground kind without the copy, nil off the board; good until the cells change.
func (t *Cells) KindOf(c cell.ID) *cell.Kind {
	if t.store == nil {
		k := t.seed.Kind(c)
		return &k
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return nil
	}
	return &t.store.groundOf(i).Kind
}

// Set assigns c's ground kind.
func (t *Cells) Set(c cell.ID, kind cell.Kind) {
	if t.store == nil {
		before := t.seed.Version()
		if t.seed.Set(c, kind); t.seed.Version() != before {
			t.touch(c)
			t.version++
		}
		return
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return
	}
	if g := t.store.groundOf(i); g.Kind != kind {
		g.Kind = kind
		t.touch(c)
		t.version++
	}
}

// SetAll assigns kind to every cell's ground.
func (t *Cells) SetAll(kind cell.Kind) {
	t.touchAll()
	if t.store == nil {
		t.seed.SetAll(kind)
		return
	}
	t.store.setAll(kind)
	t.version++
}

// Way is what runs across c; the zero Way off the board or where nothing does.
func (t *Cells) Way(c cell.ID) cell.Way {
	if t.store == nil {
		return t.seed.Ways[c]
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Way{}
	}
	return *t.store.wayOf(i)
}

// SetWay lays w across c, the zero Way taking what ran there away.
func (t *Cells) SetWay(c cell.ID, w cell.Way) {
	if t.store == nil {
		before := t.seed.Version()
		if t.seed.SetWay(c, w); t.seed.Version() != before {
			t.touch(c)
			t.version++
		}
		return
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return
	}
	if p := t.store.wayOf(i); *p != w {
		*p = w
		t.touch(c)
		t.version++
	}
}

// Crossing is what crosses c over its Way; the zero Crossing off the board or where nothing does.
func (t *Cells) Crossing(c cell.ID) cell.Crossing {
	if t.store == nil {
		return t.seed.Crossings[c]
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Crossing{}
	}
	return *t.store.crossingOf(i)
}

// SetCrossing lays x across c over its Way, the zero Crossing taking what crossed there away.
func (t *Cells) SetCrossing(c cell.ID, x cell.Crossing) {
	if t.store == nil {
		before := t.seed.Version()
		if t.seed.SetCrossing(c, x); t.seed.Version() != before {
			t.touch(c)
			t.version++
		}
		return
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return
	}
	if p := t.store.crossingOf(i); *p != x {
		*p = x
		t.touch(c)
		t.version++
	}
}

// Tags are the game's tags of places c carries; none off the board.
func (t *Cells) Tags(c cell.ID) cell.Tags {
	if t.store == nil {
		return t.seed.Tags[c]
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return 0
	}
	return t.store.tagsOf(i)
}

// States are the markers of the effects on c now; none before the cells are made.
func (t *Cells) States(c cell.ID) tag.Tags[effect.States] {
	if t.store == nil {
		return 0
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return 0
	}
	return t.store.statesOf(i)
}

// Tag gives c the tags for good; only the seed takes them, before the cells are made.
func (t *Cells) Tag(c cell.ID, tags cell.Tags) {
	if t.store != nil {
		panic("board: a cell's tags are given in the Layout, before the cells are made")
	}
	t.seed.Tag(c, tags)
}

// Cast has c play roles for good; only the seed takes them, before the cells are made.
func (t *Cells) Cast(c cell.ID, roles tag.Tags[rule.Roles]) {
	if t.store != nil {
		panic("board: a cell's roles are given in the Layout, before the cells are made")
	}
	t.seed.Cast(c, roles)
}

// Wire wires c to w for good; only the seed takes it, before the cells are made.
func (t *Cells) Wire(c cell.ID, w *rule.Wire) {
	if t.store != nil {
		panic("board: a cell's wire is given in the Layout, before the cells are made")
	}
	t.seed.Wire(c, w)
}

// CellVersion counts the changes to c; it only grows, and changes to other cells leave it as it is.
func (t *Cells) CellVersion(c cell.ID) uint64 {
	i, ok := t.Ordinal(c)
	if !ok || i >= len(t.stamps) {
		return t.everyone
	}
	return max(t.stamps[i], t.everyone)
}

// Changes counts the changes to the cells, one at a time or all at once.
func (t *Cells) Changes() uint64 { return t.changes }

// Version counts the changes to the terrain, written or by an effect; it starts over with a load.
func (t *Cells) Version() uint64 {
	if t.store == nil {
		return t.version + t.seed.Version()
	}
	return t.version
}

// Touch counts a change to c made beyond the terrain: its heights.
func (t *Cells) Touch(c cell.ID) {
	t.touch(c)
	t.version++
}

// touch counts a change to c.
func (t *Cells) touch(c cell.ID) {
	if i, ok := t.Ordinal(c); ok && i < len(t.stamps) {
		t.changes++
		t.stamps[i] = t.changes
	}
}

// touchAll counts a change to every cell at once.
func (t *Cells) touchAll() {
	t.changes++
	t.everyone = t.changes
}

// made hands the terrain over to the cells' entities in st.
func (t *Cells) made(st *store) {
	t.version += t.seed.Version()
	t.store, t.seed = st, nil
	t.touchAll()
}
