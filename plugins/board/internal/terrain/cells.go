package terrain

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/grids"
	"github.com/kjkrol/uid"
)

// Cells is a board's cells' terrain: its seed until the ECS is set up, then the cells' entities,
// with the counts of changes to them. Not safe for concurrent use.
type Cells struct {
	grid      grid.Grid
	square    uint64 // a square grid's cell count; 0 for any other grid
	seed      *cell.TerrainMap
	store     *store // nil before Setup
	version   uint64
	stamps    []uint64 // by ordinal: the count of changes when the cell last changed
	changes   uint64
	everyone  uint64 // the count when every cell last changed at once
	built     int    // a board of none seeks none
	railed    int
	bands     int
	narrowest float64
	thinnest  float64
	reached   []bool // by ordinal: a neighbour's band reaches into the cell
	bridges   Bridges
	fresh     bool // made afresh, not found after a load
	decks     []Deck
	rails     []Railing
	outline   []geom.Vec
	standing  *builtSystem
}

// New is the terrain of g's cells, every one of the zero kind until it is set.
func New(g grid.Grid) *Cells {
	t := &Cells{grid: g, seed: cell.NewTerrainMap(), stamps: make([]uint64, g.CellCount())}
	if sq, ok := g.(*grids.Square); ok {
		t.square = uint64(sq.Width) * uint64(sq.Height)
	}
	return t
}

// System makes an entity for every cell at Setup, each with template's defaults, or finds those
// a save brought back. It counts the changes effects make to them.
func (t *Cells) System(template *kind.Template,
	mattering func() tag.Tags[effect.States]) goke.System {
	return &entitySystem{cells: t, template: template, mattering: mattering}
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

// Kind is c's kind as whoever crosses it meets it: the ground's under its Way and what is built
// over it; off the board, the zero kind.
func (t *Cells) Kind(c cell.ID) cell.Kind {
	if t.store == nil {
		return t.seed.Ways[c].Over(t.seed.Kind(c))
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Kind{}
	}
	k := t.store.wayOf(i).Over(t.store.groundOf(i).Kind)
	if t.built > 0 {
		if b := t.store.builtOf(i); b != nil {
			k = b.Over(k)
		}
	}
	return k
}

// KindAt is c's kind on its deck or under it where a raised Floor gives it two levels; on a cell
// of one level, Kind.
func (t *Cells) KindAt(c cell.ID, deck bool) cell.Kind {
	b := t.Built(c)
	if b.Floor.Height <= 0 {
		return t.Kind(c)
	}
	i, _ := t.Ordinal(c)
	return level(t.store.wayOf(i).Over(t.store.groundOf(i).Kind), b, deck)
}

// KindFor is c's kind at p for whoever stands with its feet at feet and steps reach high: a Floor
// over p within its reach, else the ground under the decks. ground gives a cell's level.
func (t *Cells) KindFor(c cell.ID, p geom.Vec, feet, reach float64,
	ground func(cell.ID) float64) cell.Kind {
	if t.store == nil {
		return t.seed.Ways[c].Over(t.seed.Kind(c))
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Kind{}
	}
	k := t.store.wayOf(i).Over(t.store.groundOf(i).Kind)
	if t.built == 0 {
		return k
	}
	b := t.store.builtOf(i)
	switch {
	case b == nil:
		return k
	case b.Floor.Allows == 0 && (t.bands == 0 || !t.reached[i]):
		return b.Over(k)
	}
	t.decks = t.DecksOver(c, p, t.decks[:0])
	for _, d := range t.decks {
		f := d.Built.Floor
		raised := f.Height > 0 && ground != nil
		if raised && feet < ground(d.Cell)+f.Height-reach {
			continue
		}
		on := cell.Built{Floor: f, Wall: b.Wall}
		if raised {
			return level(k, on, true)
		}
		return on.Over(k)
	}
	return level(k, *b, false)
}

// CellFor is the cell whoever stands at p with its feet at feet and steps reach high is on: the
// highest Floor over p within its reach, else the cell under p. ground gives a cell's level.
func (t *Cells) CellFor(p geom.Vec, feet, reach float64,
	ground func(cell.ID) float64) (cell.ID, bool) {
	c, _, ok := t.LevelFor(p, feet, reach, ground)
	return c, ok
}

// LevelFor is CellFor and whether whoever stands there stands on a raised deck.
func (t *Cells) LevelFor(p geom.Vec, feet, reach float64,
	ground func(cell.ID) float64) (cell.ID, bool, bool) {
	c, ok := t.grid.CellAt(p)
	if !ok || t.built == 0 || t.store == nil {
		return c, false, ok
	}
	t.decks = t.DecksOver(c, p, t.decks[:0])
	best, top, raised := c, math.Inf(-1), false
	for _, d := range t.decks {
		f := d.Built.Floor
		h := f.Height
		if ground != nil {
			h += ground(d.Cell)
		}
		if f.Height > 0 && ground != nil && feet < h-reach || h <= top {
			continue
		}
		best, top, raised = d.Cell, h, f.Height > 0
	}
	return best, raised, true
}

func level(k cell.Kind, b cell.Built, deck bool) cell.Kind {
	floor := b.Floor
	b.Floor = cell.Floor{}
	if !deck {
		return b.Over(k)
	}
	on := cell.Kind{Name: k.Name, Cost: k.Cost, SpriteID: k.SpriteID}
	if floor.Cost > 0 {
		on.Cost = floor.Cost
	}
	on.Allows, on.Graded = floor.Allows, floor.Graded
	return b.Over(on)
}

// Standing calls each with every entity standing on c (cell.On); none before the cells are made.
func (t *Cells) Standing(c cell.ID, each func(uid.UID64)) {
	if t.standing != nil {
		t.standing.standing(c, each)
	}
}

// Built is what the entities standing on c build over it; none off the board or before the cells
// are made.
func (t *Cells) Built(c cell.ID) cell.Built {
	if t.built == 0 || t.store == nil {
		return cell.Built{}
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Built{}
	}
	if b := t.store.builtOf(i); b != nil {
		return *b
	}
	return cell.Built{}
}

// Bare is c's ground kind, bare of what runs across it — a way, a bridge — with what else is built
// over it; off the board, the zero kind.
func (t *Cells) Bare(c cell.ID) cell.Kind {
	k := t.KindOf(c)
	if k == nil {
		return cell.Kind{}
	}
	b := t.Built(c)
	if b.Floor.Graded {
		b.Floor = cell.Floor{} // a bridge runs across as a way does, the ground beside it bare
	}
	return b.Over(*k)
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

// Cast has c play roles for good; only the seed takes them, before the cells are made.
func (t *Cells) Cast(c cell.ID, roles tag.Tags[cond.Roles]) {
	if t.store != nil {
		panic("board: a cell's roles are given in the Layout, before the cells are made")
	}
	t.seed.Cast(c, roles)
}

// Label calls c as l says for good; only the seed takes it, before the cells are made.
func (t *Cells) Label(c cell.ID, l ceremony.Label) {
	if t.store != nil {
		panic("board: a cell's name and group are given in the Layout, before the cells are made")
	}
	t.seed.Label(c, l)
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

func (t *Cells) touch(c cell.ID) {
	if i, ok := t.Ordinal(c); ok && i < len(t.stamps) {
		t.changes++
		t.stamps[i] = t.changes
	}
}

func (t *Cells) touchAll() {
	t.changes++
	t.everyone = t.changes
}

func (t *Cells) made(st *store) {
	t.version += t.seed.Version()
	t.store, t.seed = st, nil
	t.touchAll()
}
