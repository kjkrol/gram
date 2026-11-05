package world

import (
	"cmp"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// Aggregate marks the kind of an entity standing for parts, a forest's trees or an army's soldiers:
// they are the truth finer than Strategic, spawned by its rules at its Unfold, and it alone from
// Strategic on, as they are summed into it at its Fold.
type Aggregate struct{}

// Field marks an aggregate standing over a field, a forest: the box of Size round its centre,
// each sector of it unfolding and folding apart. Its own box, within the world's sizes, is where
// it stands.
type Field struct{ Size geom.Vec }

// PartOf is a part's aggregate, the Whole it is summed into as that folds; SpawnPart puts it on
// the part it spawns.
type PartOf struct{ Whole uid.UID64 }

// Unfolded marks an aggregate some stretch of which is unfolded, from its first Unfold to its
// last Fold: its parts stand for it there, and it is neither drawn nor moved for itself.
type Unfolded struct{}

// a stretch of an aggregate unfolded: its sector, the aggregate within it (a point aggregate's one
// stretch following its centre) and the clock's time it folds at, zero while that sector is finer
// than L4; Folded once its Fold is told, kept the step its parts go so none draws it beside them
type stretch struct {
	Whole  uid.UID64
	Sector int
	Box    geom.AABB
	Folds  time.Duration
	Folded bool
}

// two aggregates on the abstract graph whose boxes overlapped at the last step, A the lesser
type meeting struct{ A, B uid.UID64 }

// past so many aggregates on the abstract graph a world not wrapping in x sweeps them by x, not
// pairing each with every other
const sweepPast = 64

// tells the aggregates their Unfold and Fold as the sectors of their stretches cross between L3
// and L4, their parts a Folding, and those on the abstract graph whom they Met
type wholes struct {
	w *module

	unfolds  host.Rules[Unfold]
	folds    host.Rules[Fold]
	foldings host.Rules[Folding]
	meets    host.PairRules[Met]

	all        *goke.Query
	cols       *cgoke.Columns
	base       goke.Comp[Base]
	field      goke.OptComp[Field]
	unfolded   goke.OptComp[Unfolded]
	unfoldedID goke.CompID
	stretches  *goke.Query
	stretch    goke.Comp[stretch]
	// the aggregates carrying Unfolded
	unfoldedAll *goke.Query
	unfoldedOf  goke.Comp[Base]
	opening     *goke.Factory
	opened      goke.Comp[stretch]
	// asleep too: a fold finds them by their whole
	parts    *goke.Query
	partCols *cgoke.Columns
	partOf   goke.Comp[PartOf]
	partBase goke.Comp[Base]

	abstract *goke.Query
	absCols  *cgoke.Columns
	absBase  goke.Comp[Base]
	meetings *goke.Query
	met      goke.Comp[meeting]
	making   *goke.Factory
	made     goke.Comp[meeting]

	// a step's stretches standing, by whole and sector, and those to make as they unfold
	holding   []holding
	unfolding []piece
	cover     []sector.Part
	// the aggregates Unfolded comes off this step
	marking []uid.UID64
	// a step's stretches folding, by whole and sector, found by a field's place or a point's whole
	folding []folding
	byPlace map[place]int
	byWhole map[uid.UID64]int
	bodies  []body
	pairs   []pair
	walk    struct {
		ids   []uid.UID64
		bases []Base
		// by the i-th walked: where its moments begin, how many, and which of them is told now
		from, n []int
		round   int
		// by the i-th part walked: the stretch it folds with, -1 none
		with []int
	}
	// bound once, so a step allocates no method value
	unfoldAt   func(i int) Unfold
	unfoldsAt  func(i int) bool
	foldAt     func(i int) Fold
	foldsAt    func(i int) bool
	foldingAt  func(i int) Folding
	foldedInAt func(i int) bool
	byIDs      func(p, q pair) int
	toMeeting  func(p pair, m meeting) int
}

// a stretch standing as a step finds it: walked where its aggregate was, its sector at L3 or finer
// where fine, its place there as the walk found it; folded, told its Fold the step before
type holding struct {
	id     uid.UID64
	st     *stretch
	whole  uid.UID64
	sector int
	box    geom.AABB
	field  bool
	walked bool
	fine   bool
	folded bool
}

// a stretch unfolding, and the entity made for it
type piece struct {
	whole   uid.UID64
	sector  int
	box     geom.AABB
	stretch uid.UID64
}

// a stretch folding, with the parts the step finds in it and, a point aggregate's, the sum of the
// ways from its centre to theirs
type folding struct {
	whole  uid.UID64
	sector int
	box    geom.AABB
	field  bool
	parts  int
	ways   geom.Vec
}

type place struct {
	whole  uid.UID64
	sector int
}

// an aggregate on the abstract graph as a step finds it
type body struct {
	id    uid.UID64
	box   plane.AABB
	marks host.Marks
}

// two bodies overlapping, by their places in the step's bodies, a the one of the lesser id
type pair struct {
	a, b  int
	known bool
}

var _ goke.System = (*wholes)(nil)

func newWholes(w *module) *wholes {
	s := &wholes{w: w, byPlace: map[place]int{}, byWhole: map[uid.UID64]int{}}
	s.unfoldAt, s.unfoldsAt = s.unfoldOf, s.told
	s.foldAt, s.foldsAt = s.foldOf, s.told
	s.foldingAt, s.foldedInAt = s.foldingOf, s.foldsWith
	s.byIDs, s.toMeeting = s.pairsByIDs, s.pairToMeeting
	return s
}

func (s *wholes) Init(si *goke.SysInit) {
	si.RegComp[Aggregate]()
	si.RegComp[Field]()
	si.RegComp[PartOf]()
	s.unfoldedID = si.RegComp[Unfolded]()
	steady := s.w.p.Pace(lod.Steady)
	qb := si.NewQueryBuilder(&s.base).Include(goke.Include[Aggregate]()).Optional(&s.field).
		Optional(&s.unfolded)
	s.cols = cgoke.NewColumns(qb)
	s.unfolds.Bind(s.cols)
	s.folds.Bind(s.cols)
	s.all = qb.With(steady).Build()
	s.stretches = si.NewQueryBuilder(&s.stretch).Build()
	s.unfoldedAll = si.NewQueryBuilder(&s.unfoldedOf).
		Include(goke.Include[Aggregate](), goke.Include[Unfolded]()).Build()
	s.opening = si.NewFactory(&s.opened)
	pb := si.NewQueryBuilder(&s.partOf, &s.partBase)
	s.partCols = cgoke.NewColumns(pb)
	s.foldings.Bind(s.partCols)
	s.parts = pb.Build()
	ab := si.NewQueryBuilder(&s.absBase).
		Include(goke.Include[Aggregate](), goke.Include[sector.Abstract]())
	s.absCols = cgoke.NewColumns(ab)
	s.meets.Bind(s.absCols)
	s.abstract = ab.With(steady).Build()
	s.meetings = si.NewQueryBuilder(&s.met).Build()
	s.making = si.NewFactory(&s.made)
}

func (s *wholes) Update(cb *goke.CmdBuf, d time.Duration) {
	t := s.w.tick(d)
	s.unfold(cb, d, t)
	if len(s.folding) > 0 {
		s.fold(t)
	}
	if !s.meets.Empty() {
		s.meet(cb, d, t)
	}
}

// finds the stretches of the aggregates, waits out those standing at L4 or coarser, noting for fold
// those whose wait is over, and tells Unfold of those come down to L3 or finer with none standing
// yet; the wait begins with the step, the sectors laid as the tick began
func (s *wholes) unfold(cb *goke.CmdBuf, d time.Duration, t host.Tick) {
	s.holding, s.unfolding, s.folding = s.holding[:0], s.unfolding[:0], s.folding[:0]
	for s.stretches.All(); s.stretches.Next(); {
		cur := s.stretches.Cursor()
		sts := s.stretch.Slice(cur)
		for j := range sts {
			st := &sts[j]
			s.holding = append(s.holding, holding{id: cur.IDs[j], st: st, whole: st.Whole,
				sector: st.Sector, box: st.Box, folded: st.Folded})
		}
	}
	slices.SortFunc(s.holding, holdingByPlace)
	// a point aggregate goes at its own sector's pace; a field is walked asleep too, as the others
	// of its sectors may be awake
	for s.all.Paced(d); s.all.Next(); {
		if cur := s.all.Cursor(); !s.field.Present(cur) {
			s.find(cur, nil)
		}
	}
	for s.all.All(); s.all.Next(); {
		if cur := s.all.Cursor(); s.field.Present(cur) {
			s.find(cur, s.field.Slice(cur))
		}
	}
	s.wait(cb, d, t)
	s.open(t)
	s.mark(cb)
}

// finds the stretches of the aggregates of the block cur, a point aggregate's one in the sector of
// its centre, a field's, of the sizes given, in each sector it covers: those standing are walked,
// those come to L3 or finer with none standing but one folded the step before unfold
func (s *wholes) find(cur *goke.Cursor, fields []Field) {
	sectors := s.w.sectors
	for i, b := range s.base.Slice(cur) {
		id := cur.IDs[i]
		if _, gone := s.w.despawned[id]; gone {
			continue
		}
		c := b.Pos.Center()
		lo, hi := s.holdingOf(id)
		if fields == nil {
			at := sectors.of(c.X, c.Y)
			box := geom.NewAABB(b.Pos.TopLeft, b.Pos.TopLeft.Add(b.Pos.Size))
			fine := sectors.levelOf(at) < lod.Strategic
			if lo < hi && !s.holding[lo].folded {
				h := &s.holding[lo]
				h.walked, h.fine, h.sector, h.box = true, fine, at, box
			} else if fine {
				s.unfolding = append(s.unfolding, piece{whole: id, sector: at, box: box})
			}
			continue
		}
		for k := lo; k < hi; k++ {
			s.holding[k].walked, s.holding[k].field = true, true
		}
		half := geom.NewVec(fields[i].Size.X/2, fields[i].Size.Y/2)
		s.cover = sectors.cover(geom.NewAABB(c.Sub(half), c.Add(half)), s.cover[:0])
		for _, p := range s.cover {
			fine := sectors.levelOf(p.Sector) < lod.Strategic
			if k := s.holdingIn(lo, hi, p.Sector); k >= 0 && !s.holding[k].folded {
				s.holding[k].fine, s.holding[k].box = fine, p.Box
			} else if fine {
				s.unfolding = append(s.unfolding, piece{whole: id, sector: p.Sector, box: p.Box})
			}
		}
	}
}

// where the stretches standing of whole lie among the step's
func (s *wholes) holdingOf(whole uid.UID64) (lo, hi int) {
	if len(s.holding) == 0 {
		return 0, 0
	}
	lo, _ = slices.BinarySearchFunc(s.holding, place{whole: whole, sector: -1}, holdingToPlace)
	for hi = lo; hi < len(s.holding) && s.holding[hi].whole == whole; hi++ {
	}
	return lo, hi
}

// the stretch among [lo, hi), a field's, standing in sector; -1 for none
func (s *wholes) holdingIn(lo, hi, sector int) int {
	if lo == hi {
		return -1
	}
	k, ok := slices.BinarySearchFunc(s.holding[lo:hi], place{whole: s.holding[lo].whole,
		sector: sector}, holdingToPlace)
	if !ok {
		return -1
	}
	return lo + k
}

// walks the stretches standing: one fine waits no more, one coarse or past its field begins its
// wait or, the wait over, folds; one folded the step before goes with its parts, as does one
// whose aggregate is gone; a point aggregate's asleep waits on
func (s *wholes) wait(cb *goke.CmdBuf, d time.Duration, t host.Tick) {
	begun, wait := t.Time-d, s.w.config.LOD.FoldAfter()
	for k := range s.holding {
		h := &s.holding[k]
		if h.folded {
			cb.RemoveOne(h.id)
			continue
		}
		if !h.walked {
			if _, gone := s.w.despawned[h.whole]; gone || !s.all.Seek(h.whole) {
				cb.RemoveOne(h.id)
			}
			continue
		}
		st := h.st
		st.Sector, st.Box = h.sector, h.box
		switch {
		case h.fine:
			st.Folds = 0
		case st.Folds == 0:
			st.Folds = begun + wait
		case t.Time >= st.Folds:
			s.folding = append(s.folding, folding{whole: h.whole, sector: h.sector, box: h.box,
				field: h.field})
			st.Folded = true
		}
	}
}

// puts Unfolded on each aggregate a stretch of which unfolds this step, and takes it off each that
// had none standing as the step began, the step after its last folded, as its parts go
func (s *wholes) mark(cb *goke.CmdBuf) {
	for _, p := range s.unfolding {
		if s.all.Seek(p.whole) && !s.unfolded.Present(s.all.Cursor()) {
			cb.AddOne(p.whole, s.unfoldedID, Unfolded{})
		}
	}
	s.marking = s.marking[:0]
	for s.unfoldedAll.All(); s.unfoldedAll.Next(); {
		for _, id := range s.unfoldedAll.Cursor().IDs {
			if !s.stands(id) && !s.unfoldingOf(id) {
				s.marking = append(s.marking, id)
			}
		}
	}
	for _, id := range s.marking {
		if _, gone := s.w.despawned[id]; !gone {
			cb.RemoveCompOne(id, s.unfoldedID)
		}
	}
}

// whether a stretch of whole stood as the step began, but one folded the step before
func (s *wholes) stands(whole uid.UID64) bool {
	lo, hi := s.holdingOf(whole)
	for k := lo; k < hi; k++ {
		if !s.holding[k].folded {
			return true
		}
	}
	return false
}

// whether a stretch of whole unfolds this step
func (s *wholes) unfoldingOf(whole uid.UID64) bool {
	for _, p := range s.unfolding {
		if p.whole == whole {
			return true
		}
	}
	return false
}

// makes a stretch for each unfolding, then tells their aggregates their Unfold
func (s *wholes) open(t host.Tick) {
	if len(s.unfolding) == 0 {
		return
	}
	s.opening.Create(len(s.unfolding))
	n := 0
	for s.opening.Next() {
		made := s.opened.Slice(&s.opening.Cursor)
		for i, id := range s.opening.IDs {
			p := &s.unfolding[n]
			p.stretch = id
			made[i] = stretch{Whole: p.whole, Sector: p.sector, Box: p.box}
			n++
		}
	}
	if s.unfolds.Empty() {
		return
	}
	slices.SortFunc(s.unfolding, piecesByPlace)
	for s.all.All(); s.all.Next(); {
		cur := s.all.Cursor()
		s.walk.bases = s.base.Slice(cur)
		most := s.unfoldsOf(cur.IDs)
		for s.walk.round = 0; s.walk.round < most; s.walk.round++ {
			s.unfolds.RunWhere(t, s.cols.Row(cur), len(cur.IDs), s.unfoldsAt, s.unfoldAt)
		}
	}
}

// counts the parts of the stretches folding, telling each a Folding, moves each point aggregate
// to its parts' centroid, then tells the aggregates their Fold
func (s *wholes) fold(t host.Tick) {
	slices.SortFunc(s.folding, foldingsByPlace)
	clear(s.byPlace)
	clear(s.byWhole)
	for k, f := range s.folding {
		if f.field {
			s.byPlace[place{whole: f.whole, sector: f.sector}] = k
		} else {
			s.byWhole[f.whole] = k
		}
	}
	for s.parts.All(); s.parts.Next(); {
		cur := s.parts.Cursor()
		of, bases := s.partOf.Slice(cur), s.partBase.Slice(cur)
		s.walk.ids, s.walk.with = cur.IDs, s.walk.with[:0]
		told := false
		for i, id := range cur.IDs {
			c := bases[i].Pos.Center()
			k := s.foldsWithin(id, of[i].Whole, c)
			s.walk.with = append(s.walk.with, k)
			if k >= 0 {
				f := &s.folding[k]
				f.parts++
				if !f.field {
					f.ways = f.ways.Add(s.w.toward(middle(f.box), c))
				}
				told = true
			}
		}
		if told && !s.foldings.Empty() {
			s.foldings.RunWhere(t, s.partCols.Row(cur), len(cur.IDs), s.foldedInAt, s.foldingAt)
		}
	}
	s.gather()
	if s.folds.Empty() {
		return
	}
	for s.all.All(); s.all.Next(); {
		cur := s.all.Cursor()
		most := s.foldsOf(cur.IDs)
		for s.walk.round = 0; s.walk.round < most; s.walk.round++ {
			s.folds.RunWhere(t, s.cols.Row(cur), len(cur.IDs), s.foldsAt, s.foldAt)
		}
	}
}

// moves each point aggregate folding with parts to their centroid, the space gathering its runs
// anew; a field stays
func (s *wholes) gather() {
	moved := false
	for k := range s.folding {
		f := &s.folding[k]
		if f.field || f.parts == 0 || !s.all.Seek(f.whole) {
			continue
		}
		n := float32(f.parts)
		s.w.space.Move(&s.base.At(s.all.Cursor()).Pos.AABB, geom.NewVec(f.ways.X/n, f.ways.Y/n))
		moved = true
	}
	if moved {
		s.w.index(s.w.indexer.query, &s.w.indexer.base)
	}
}

// the stretch folding that the part id of whole, its centre at c, folds with: a point
// aggregate's wherever the part is, a field's holding c; -1 for none or a part gone
func (s *wholes) foldsWithin(id, whole uid.UID64, c geom.Vec) int {
	if _, gone := s.w.despawned[id]; gone {
		return -1
	}
	if k, ok := s.byWhole[whole]; ok {
		return k
	}
	if k, ok := s.byPlace[place{whole: whole, sector: s.w.sectors.of(c.X, c.Y)}]; ok {
		return k
	}
	return -1
}

// has the walk say where the stretches unfolding of each of ids begin and how many; it is the
// most any has, each told in a round of its own
func (s *wholes) unfoldsOf(ids []uid.UID64) (most int) {
	s.walk.from, s.walk.n = s.walk.from[:0], s.walk.n[:0]
	for _, id := range ids {
		k, _ := slices.BinarySearchFunc(s.unfolding, id, pieceToWhole)
		n := 0
		for k+n < len(s.unfolding) && s.unfolding[k+n].whole == id {
			n++
		}
		s.walk.from, s.walk.n = append(s.walk.from, k), append(s.walk.n, n)
		most = max(most, n)
	}
	return most
}

// has the walk say where the stretches folding of each of ids begin and how many; it is the most
// any has
func (s *wholes) foldsOf(ids []uid.UID64) (most int) {
	s.walk.from, s.walk.n = s.walk.from[:0], s.walk.n[:0]
	for _, id := range ids {
		k, _ := slices.BinarySearchFunc(s.folding, id, foldingToWhole)
		n := 0
		for k+n < len(s.folding) && s.folding[k+n].whole == id {
			n++
		}
		s.walk.from, s.walk.n = append(s.walk.from, k), append(s.walk.n, n)
		most = max(most, n)
	}
	return most
}

// whether the i-th walked is told a moment this round
func (s *wholes) told(i int) bool { return s.walk.round < s.walk.n[i] }

func (s *wholes) unfoldOf(i int) Unfold {
	p := &s.unfolding[s.walk.from[i]+s.walk.round]
	return Unfold{ID: p.whole, At: s.walk.bases[i].Pos.Center(), Sector: p.sector, Box: p.box,
		stretch: p.stretch}
}

func (s *wholes) foldOf(i int) Fold {
	f := &s.folding[s.walk.from[i]+s.walk.round]
	return Fold{ID: f.whole, Parts: f.parts, Sector: f.sector, Box: f.box}
}

func (s *wholes) foldingOf(i int) Folding {
	f := &s.folding[s.walk.with[i]]
	return Folding{ID: s.walk.ids[i], Whole: f.whole, Sector: f.sector, Box: f.box}
}

func (s *wholes) foldsWith(i int) bool { return s.walk.with[i] >= 0 }

func holdingByPlace(a, b holding) int {
	return cmp.Or(cmp.Compare(a.whole, b.whole), cmp.Compare(a.sector, b.sector))
}

func holdingToPlace(h holding, at place) int {
	return cmp.Or(cmp.Compare(h.whole, at.whole), cmp.Compare(h.sector, at.sector))
}

func piecesByPlace(a, b piece) int {
	return cmp.Or(cmp.Compare(a.whole, b.whole), cmp.Compare(a.sector, b.sector))
}

func pieceToWhole(p piece, whole uid.UID64) int { return cmp.Compare(p.whole, whole) }

func foldingsByPlace(a, b folding) int {
	return cmp.Or(cmp.Compare(a.whole, b.whole), cmp.Compare(a.sector, b.sector))
}

func foldingToWhole(f folding, whole uid.UID64) int { return cmp.Compare(f.whole, whole) }

// the middle of box
func middle(box geom.AABB) geom.Vec {
	return geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2)
}

// the way from a to b, the shorter way round an axis of the world that wraps
func (w *module) toward(a, b geom.Vec) geom.Vec {
	d := b.Sub(a)
	sp := w.config.Space
	if sp.Edges.WrapsX() {
		d.X = f32.Remainder(d.X, float32(sp.Width))
	}
	if sp.Edges.WrapsY() {
		d.Y = f32.Remainder(d.Y, float32(sp.Height))
	}
	return d
}

// tells Met to the pairs of aggregates on the abstract graph overlapping now and not at the last
// step, a meeting entity remembering each such pair while they overlap, a save too
func (s *wholes) meet(cb *goke.CmdBuf, d time.Duration, t host.Tick) {
	s.bodies = s.bodies[:0]
	for s.abstract.Paced(d); s.abstract.Next(); {
		cur := s.abstract.Cursor()
		bases, row := s.absBase.Slice(cur), s.absCols.Row(cur)
		for i, id := range cur.IDs {
			if _, gone := s.w.despawned[id]; !gone {
				s.bodies = append(s.bodies,
					body{id: id, box: bases[i].Pos.AABB, marks: s.meets.Read(0, row, i)})
			}
		}
	}
	s.overlapping()
	for s.meetings.All(); s.meetings.Next(); {
		cur := s.meetings.Cursor()
		for i, m := range s.met.Slice(cur) {
			if k, ok := slices.BinarySearchFunc(s.pairs, m, s.toMeeting); ok {
				s.pairs[k].known = true
			} else {
				cb.RemoveOne(cur.IDs[i])
			}
		}
	}
	fresh := 0
	for _, p := range s.pairs {
		if !p.known {
			a, b := &s.bodies[p.a], &s.bodies[p.b]
			s.meets.DispatchEitherWay(t, a.marks, b.marks, Met{A: a.id, B: b.id},
				Met{A: b.id, B: a.id})
			fresh++
		}
	}
	if fresh == 0 {
		return
	}
	s.making.Create(fresh)
	k := 0
	for s.making.Next() {
		made := s.made.Slice(&s.making.Cursor)
		for i := range s.making.IDs {
			for s.pairs[k].known {
				k++
			}
			made[i] = meeting{A: s.bodies[s.pairs[k].a].id, B: s.bodies[s.pairs[k].b].id}
			k++
		}
	}
}

// has the step's pairs of bodies overlapping, sorted by their ids
func (s *wholes) overlapping() {
	s.pairs = s.pairs[:0]
	n := len(s.bodies)
	sweep := n > sweepPast && !s.w.config.Space.Edges.WrapsX()
	if sweep {
		slices.SortFunc(s.bodies, byLeft)
	}
	for i := range n {
		for j := i + 1; j < n; j++ {
			if sweep && s.bodies[j].box.TopLeft.X > s.bodies[i].box.BottomRight.X {
				break
			}
			if _, ok := s.bodies[i].box.DeepestOverlapWith(&s.bodies[j].box); !ok {
				continue
			}
			if s.bodies[i].id < s.bodies[j].id {
				s.pairs = append(s.pairs, pair{a: i, b: j})
			} else {
				s.pairs = append(s.pairs, pair{a: j, b: i})
			}
		}
	}
	slices.SortFunc(s.pairs, s.byIDs)
}

func byLeft(a, b body) int { return cmp.Compare(a.box.TopLeft.X, b.box.TopLeft.X) }

func (s *wholes) pairsByIDs(p, q pair) int {
	return cmp.Or(cmp.Compare(s.bodies[p.a].id, s.bodies[q.a].id),
		cmp.Compare(s.bodies[p.b].id, s.bodies[q.b].id))
}

func (s *wholes) pairToMeeting(p pair, m meeting) int {
	return cmp.Or(cmp.Compare(s.bodies[p.a].id, m.A), cmp.Compare(s.bodies[p.b].id, m.B))
}
