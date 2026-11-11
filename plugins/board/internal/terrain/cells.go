package terrain

import (
	"encoding/binary"
	"hash/fnv"
	"hash/maphash"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
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
	everyone  uint64   // the count when every cell last changed at once
	passage   []uint64 // by ordinal: the hash of the cell's passage as it last counted
	passed    []uint64 // by ordinal: the count of passages when the cell's last changed
	passages  uint64
	allPassed uint64 // the count when every cell's passage last changed at once
	hashing   maphash.Seed
	built     int // a board of none seeks none
	railed    int
	bands     int
	sunk      int // the floors sunk under the ground
	storeyed  int // the floors raised or sunk: the levels beside the ground
	narrowest float32
	thinnest  float32
	reached   []bool // by ordinal: a neighbour's band reaches into the cell
	limits    []bool // by ordinal: a floor's Load or a wall's Gap limits whom the cell takes
	limited   []int  // the ordinals limits holds true
	bridges   Bridges
	fresh     bool // made afresh, not found after a load
	outline   []geom.Vec
	standing  *builtSystem
	read      Read
	forks     []*Read
	changed   func(c cell.ID) // after c changed
	all       func()          // after every cell changed
}

// New is the terrain of g's cells, every one of the zero kind until it is set.
func New(g grid.Grid) *Cells {
	n := g.CellCount()
	t := &Cells{grid: g, seed: cell.NewTerrainMap(), stamps: make([]uint64, n),
		passage: make([]uint64, n), passed: make([]uint64, n), hashing: maphash.MakeSeed()}
	t.read.t = t
	if sq, ok := g.(*grids.Square); ok {
		t.square = uint64(sq.Width) * uint64(sq.Height)
	}
	bare := maphash.Comparable(t.hashing, passage{})
	for i := range t.passage {
		t.passage[i] = bare
	}
	return t
}

// System makes an entity for every cell at Setup, each with what roster asks of template, or finds
// those a save brought back. It counts the changes effects make to them.
func (t *Cells) System(roster *kind.Roster, template *kind.Template,
	mattering func() tag.Tags[effect.States]) goke.System {
	return &entitySystem{cells: t, roster: roster, template: template, mattering: mattering}
}

// Notify has changed called after a cell changes, all after every cell does at once.
func (t *Cells) Notify(changed func(c cell.ID), all func()) { t.changed, t.all = changed, all }

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
func (r *Read) Entity(c cell.ID) (uid.UID64, bool) {
	t := r.t
	i, ok := t.Ordinal(c)
	if !ok || r.store == nil {
		return 0, false
	}
	return r.store.ids[i], true
}

// Kind is c's kind as whoever crosses it meets it: the ground's under its Way and what is built
// over it; off the board, the zero kind.
func (r *Read) Kind(c cell.ID) cell.Kind {
	t := r.t
	if r.store == nil {
		return t.seed.Ways[c].Over(t.seed.Kind(c))
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Kind{}
	}
	k := r.store.wayOf(i).Over(r.store.groundOf(i).Kind)
	if t.built > 0 {
		if b := r.store.builtOf(i); b != nil {
			k = b.Over(k)
		}
	}
	return k
}

// KindAt is c's kind on level where a raised Floor gives it more than one; on a cell of one
// level, Kind.
func (r *Read) KindAt(c cell.ID, on cell.Level) cell.Kind {
	t := r.t
	b := r.Built(c)
	if !b.Levelled() {
		return r.Kind(c)
	}
	i, _ := t.Ordinal(c)
	return level(r.store.wayOf(i).Over(r.store.groundOf(i).Kind), b, on)
}

// KindFor is c's kind at p for whoever stands with its feet at feet and steps reach high: a Floor
// over p within its reach, else the ground under the decks. ground gives a cell's level.
func (r *Read) KindFor(c cell.ID, p geom.Vec, feet, reach float32,
	ground func(cell.ID) float32) cell.Kind {
	k, _ := r.KindOn(c, p, feet, reach, ground)
	return k
}

// KindOn is KindFor and the level the kind is on: OnDeck where a raised Floor over p is within
// reach.
func (r *Read) KindOn(c cell.ID, p geom.Vec, feet, reach float32,
	ground func(cell.ID) float32) (cell.Kind, cell.Level) {
	t := r.t
	if r.store == nil {
		return t.seed.Ways[c].Over(t.seed.Kind(c)), cell.OnGround
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Kind{}, cell.OnGround
	}
	k := r.store.wayOf(i).Over(r.store.groundOf(i).Kind)
	if t.built == 0 {
		return k, cell.OnGround
	}
	b := r.store.builtOf(i)
	switch {
	case b == nil:
		return k, cell.OnGround
	case b.Floor.Allows|b.More[0].Allows|b.More[1].Allows == 0 &&
		(t.bands == 0 || !t.reached[i]):
		return b.Over(k), cell.OnGround
	}
	// the ground beside the floors: a floor lying on it counts where p is on it
	kind, on, top := cell.Built{Wall: b.Wall, Rail: b.Rail}.Over(k), cell.OnGround, f32.Inf(-1)
	r.DecksOver(c, p, func(d Deck) bool {
		f := d.Built.Floor
		h, ok := stoodOn(f, d.Cell, feet, reach, ground)
		if !ok || h <= top {
			return true
		}
		top = h
		if built := (cell.Built{Floor: f, Wall: b.Wall}); f.Height != 0 && ground != nil {
			kind, on = levelled(k, built), f.Level()
		} else {
			kind, on = built.Over(k), cell.OnGround
		}
		return true
	})
	return kind, on
}

// CellFor is the cell whoever stands at p with its feet at feet and steps reach high is on: the
// highest Floor over p within its reach, else the cell under p. ground gives a cell's level.
func (r *Read) CellFor(p geom.Vec, feet, reach float32,
	ground func(cell.ID) float32) (cell.ID, bool) {
	c, _, ok := r.LevelFor(p, feet, reach, ground)
	return c, ok
}

// LevelFor is CellFor and the level whoever stands there stands on.
func (r *Read) LevelFor(p geom.Vec, feet, reach float32,
	ground func(cell.ID) float32) (cell.ID, cell.Level, bool) {
	t := r.t
	c, ok := t.grid.CellAt(p)
	if !ok || t.built == 0 || r.store == nil {
		return c, cell.OnGround, ok
	}
	best, top, on := c, f32.Inf(-1), cell.OnGround
	r.DecksOver(c, p, func(d Deck) bool {
		f := d.Built.Floor
		if h, ok := stoodOn(f, d.Cell, feet, reach, ground); ok && h > top {
			best, top, on = d.Cell, h, levelOf(f)
		}
		return true
	})
	return best, on, true
}

// stoodOn is how high f, built on c, lies and whether whoever has its feet at feet and steps
// reach high stands on it: a raised floor within its reach, a sunk one under whose roof its feet
// are, a floor lying on the ground whatever its feet
func stoodOn(f cell.Floor, c cell.ID, feet, reach float32,
	ground func(cell.ID) float32) (h float32, ok bool) {
	h = f.Height
	if ground == nil {
		return h, !f.Sunk()
	}
	h += ground(c)
	switch {
	case f.Height > 0:
		return h, feet >= h-reach
	case f.Sunk():
		return h, feet >= h-reach && feet < h+f.Clear
	}
	return h, true
}

// level is k as whoever crosses b's cell on l meets it: on the ground with the floors lying on
// it alone, on a storey with its floor
func level(k cell.Kind, b cell.Built, l cell.Level) cell.Kind {
	if l == cell.OnGround {
		on := cell.Built{Wall: b.Wall, Rail: b.Rail}
		if b.Floor.Height == 0 {
			on.Floor = b.Floor
		}
		return on.Over(k)
	}
	return levelled(k, cell.Built{Floor: b.On(l), Wall: b.Wall})
}

// levelled is k on b's floor, raised or sunk: a sunk floor runs under the cell's wall
func levelled(k cell.Kind, b cell.Built) cell.Kind {
	floor := b.Floor
	b.Floor = cell.Floor{}
	if floor.Sunk() {
		b.Wall = cell.Wall{}
	}
	on := cell.Kind{Name: k.Name, Pace: k.Pace, SpriteID: k.SpriteID}
	if floor.Pace > 0 {
		on.Pace = floor.Pace
	} else {
		on.Paces = k.Paces
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
func (r *Read) Built(c cell.ID) cell.Built {
	t := r.t
	if t.built == 0 || r.store == nil {
		return cell.Built{}
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Built{}
	}
	if b := r.store.builtOf(i); b != nil {
		return *b
	}
	return cell.Built{}
}

// Bare is c's ground kind, bare of what runs across it — a way, a bridge — with what else is built
// over it; off the board, the zero kind.
func (r *Read) Bare(c cell.ID) cell.Kind {
	k := r.KindOf(c)
	if k == nil {
		return cell.Kind{}
	}
	b := r.Built(c)
	if b.Floor.Graded {
		b.Floor = cell.Floor{} // a bridge runs across as a way does, the ground beside it bare
	}
	return b.Over(*k)
}

// KindOf is c's ground kind without the copy, nil off the board; good until the cells change.
func (r *Read) KindOf(c cell.ID) *cell.Kind {
	t := r.t
	if r.store == nil {
		k := t.seed.Kind(c)
		return &k
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return nil
	}
	return &r.store.groundOf(i).Kind
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
	if t.store == nil {
		t.seed.SetAll(kind)
	} else {
		t.store.setAll(kind)
		t.version++
	}
	t.touchAll()
}

// Way is what runs across c; the zero Way off the board or where nothing does.
func (r *Read) Way(c cell.ID) cell.Way {
	t := r.t
	if r.store == nil {
		return t.seed.Ways[c]
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return cell.Way{}
	}
	return *r.store.wayOf(i)
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
func (r *Read) States(c cell.ID) tag.Tags[effect.States] {
	t := r.t
	if r.store == nil {
		return 0
	}
	i, ok := t.Ordinal(c)
	if !ok {
		return 0
	}
	return r.store.statesOf(i)
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

// CellPassage is the Passage at which c's passage last changed; it only grows, and changes to
// other cells leave it as it is.
func (t *Cells) CellPassage(c cell.ID) uint64 {
	i, ok := t.Ordinal(c)
	if !ok || i >= len(t.passed) {
		return t.allPassed
	}
	return max(t.passed[i], t.allPassed)
}

// Passage counts the changes to what a step over the cells reads, one cell at a time or all at
// once: while it stays as it was, no cell's passage changed. It only grows.
func (t *Cells) Passage() uint64 { return t.passages }

// CellFingerprint is a number of what a step over c reads of it, the same on every run for the
// same cell.
func (r *Read) CellFingerprint(c cell.ID) uint64 { return r.t.CellFingerprint(c) }

// CellPassage is the Passage at which c's passage last changed; it only grows.
func (r *Read) CellPassage(c cell.ID) uint64 { return r.t.CellPassage(c) }

// Passage counts the changes to what a step over the cells reads: while it stays as it was, no
// cell's passage changed.
func (r *Read) Passage() uint64 { return r.t.passages }

// Touch counts a change to c made beyond the terrain: its heights, which a step reads.
func (t *Cells) Touch(c cell.ID) {
	if i, ok := t.Ordinal(c); ok && i < len(t.stamps) {
		t.pass(i)
		t.stamp(i, c)
	}
	t.version++
}

// touch counts a change to c's terrain, and to its passage where what a step reads of c differs
func (t *Cells) touch(c cell.ID) {
	if i, ok := t.Ordinal(c); ok && i < len(t.stamps) {
		if h := t.passageOf(i, c); h != t.passage[i] {
			t.passage[i] = h
			t.pass(i)
		}
		t.stamp(i, c)
	}
}

func (t *Cells) stamp(i int, c cell.ID) {
	t.changes++
	t.stamps[i] = t.changes
	if t.changed != nil {
		t.changed(c)
	}
}

func (t *Cells) pass(i int) {
	t.passages++
	t.passed[i] = t.passages
}

func (t *Cells) touchAll() {
	t.changes++
	t.everyone = t.changes
	t.passages++
	t.allPassed = t.passages
	t.grid.EachCell(func(c cell.ID) {
		if i, ok := t.Ordinal(c); ok && i < len(t.passage) {
			t.passage[i] = t.passageOf(i, c)
		}
	})
	if t.all != nil {
		t.all()
	}
}

// passage is what a step over a cell reads of it beyond its heights: whom its ground and the way
// across it admit and at what pace, how the way runs, what is built over it; the way's kind
// counts by its name too. A kind's Veil, Height, Sway, look and name, and a way's fading, no
// step reads.
type passage struct {
	ground, way stride
	wayName     cell.Name
	width       float32
	links       cell.Links
	built       cell.Built
}

// stride is a kind as a step meets it
type stride struct {
	allows        cell.Domain
	solid, graded bool
	pace          float64
	paces         [8]float64
}

func strideOf(k *cell.Kind) stride {
	return stride{allows: k.Allows, solid: k.Solid, graded: k.Graded, pace: k.Pace,
		paces: k.Paces}
}

// passageOf hashes the passage of c, of ordinal i, as its parts stand: read whole, not as the
// gathered counts say, which the gathering of what is built sets after it changed the cells
func (t *Cells) passageOf(i int, c cell.ID) uint64 {
	return maphash.Comparable(t.hashing, t.passageAt(i, c))
}

// CellFingerprint is a number of what a step over c reads of it, the same on every run for the
// same cell; 0 off the cells. A file kept of what was worked out over the cells is good where
// they match.
func (t *Cells) CellFingerprint(c cell.ID) uint64 {
	i, ok := t.Ordinal(c)
	if !ok {
		return 0
	}
	p := t.passageAt(i, c)
	return p.fingerprint()
}

// fingerprint is p mixed into a number, the same on every run, field by field: a stride's kind
// as a step meets it, the way, what is built
func (p *passage) fingerprint() uint64 {
	h := fnv.New64a()
	var buf [8]byte
	u64 := func(v uint64) {
		binary.LittleEndian.PutUint64(buf[:], v)
		h.Write(buf[:])
	}
	f64 := func(v float64) { u64(math.Float64bits(v)) }
	f32 := func(v float32) { u64(uint64(math.Float32bits(v))) }
	bits := func(bs ...bool) {
		var v uint64
		for i, b := range bs {
			if b {
				v |= 1 << i
			}
		}
		u64(v)
	}
	for _, s := range [2]*stride{&p.ground, &p.way} {
		u64(uint64(s.allows))
		bits(s.solid, s.graded)
		f64(s.pace)
		for _, v := range s.paces {
			f64(v)
		}
	}
	h.Write(p.wayName[:])
	f32(p.width)
	u64(uint64(p.links))
	b := &p.built
	for k := range uint8(cell.Storeys) {
		f := b.Storey(k)
		if k > 0 && *f == (cell.Floor{}) {
			continue
		}
		u64(uint64(f.Allows) | uint64(f.Links)<<8 | uint64(f.Storey)<<16)
		f32(f.Height)
		f32(f.Thickness)
		f32(f.Load)
		f64(f.Pace)
		bits(f.Graded)
		f32(f.Width)
		if k > 0 || f.Clear != 0 {
			f32(f.Clear)
		}
	}
	u64(uint64(b.Wall.Keeps))
	f32(b.Wall.Height)
	f32(b.Wall.Gap.Width)
	f32(b.Wall.Gap.Height)
	u64(uint64(b.Rail.Keeps))
	f32(b.Rail.Height)
	return h.Sum64()
}

// passageAt is the passage of c, of ordinal i, as its parts stand
func (t *Cells) passageAt(i int, c cell.ID) passage {
	var p passage
	var ground *cell.Kind
	var way cell.Way
	if t.store == nil {
		k := t.seed.Kind(c)
		ground, way = &k, t.seed.Ways[c]
	} else {
		ground, way = &t.store.groundOf(i).Kind, *t.store.wayOf(i)
		if b := t.store.builtOf(i); b != nil {
			p.built = *b
		}
	}
	p.ground, p.way, p.wayName = strideOf(ground), strideOf(&way.Kind), way.Kind.Name
	p.width, p.links = way.Width, way.Links
	return p
}

func (t *Cells) made(st *store) {
	t.version += t.seed.Version()
	t.store, t.seed, t.read.store = st, nil, st
	for _, f := range t.forks {
		f.store = st.fork()
	}
	t.touchAll()
}
