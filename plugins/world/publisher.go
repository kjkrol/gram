package world

import (
	"cmp"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// figures are the entities the renderer draws, as the last Publish settled them: the clock as
// shown, what each camera saw and every entity some camera saw.
type figures struct {
	shown   time.Duration
	clocked bool
	views   map[camera.Camera]*view.View
	all     []figure
	layers  []layer
	lies    []Lie
}

// figure is an entity drawn, a Field one a sector it covers: its box, the Field's part in that
// sector where piece, its heading and speed, its Z, its layers figures.layers[from:from+n], its
// Lie figures.lies[lie-1] (0 for none), its shadow as the Lighting says
type figure struct {
	id        uid.UID64
	box       plane.AABB
	dir       geom.Vec
	speed     float32
	z         Z
	from, lie int32
	n         uint16
	piece     bool
	shadow    render.Shadowing
}

// layer is a sprite an entity is drawn with and how far the effect its material is under has run
type layer struct {
	render.Appearance
	progress float32
}

func (s *renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z).Optional(&s.lies).
		Optional(&s.active).Optional(&s.keen).Optional(&s.asleep).Optional(&s.field).
		Optional(&s.unfolded)
	if s.lighting != nil {
		s.lighting.Join(qb)
	}
	Own(s.rules, &s.base)
	Own(s.rules, &s.appearance)
	Own(s.rules, &s.z)
	Own(s.rules, &s.active)
	Own(s.rules, &s.keen)
	s.rules.Bind(qb)
	s.renderQuery = qb.Build()
	s.stretches = si.NewQueryBuilder(&s.stretch).Build()
}

// Publish settles every entity some camera sees as the rules say, for the frame drawn next.
func (s *renderer) Publish() {
	b := s.figures.Back()
	b.all, b.layers, b.lies = b.all[:0], b.layers[:0], b.lies[:0]
	b.shown, b.clocked = 0, s.clock != nil
	if s.clock != nil {
		b.shown = s.clock()
	}
	s.seeing = s.seeing[:0]
	if b.views == nil {
		b.views = map[camera.Camera]*view.View{}
	}
	for cam, v := range s.views() {
		c := b.views[cam]
		if c == nil {
			c = &view.View{}
			b.views[cam] = c
		}
		v.CopyInto(c)
		s.seeing = append(s.seeing, c)
	}
	s.aggregate.begin()
	s.keepAll()
	if s.atlas == nil || s.renderQuery == nil || len(s.seeing) == 0 {
		return
	}
	s.found = false
	// what lies asleep at L4 is drawn by no camera, nor a point aggregate unfolded, its parts
	// alone; a field is, wherever its box is. A chunk is asked whether a camera sees it here, where
	// little is live: within publish the walk over its ids spills.
	for s.renderQuery.All(); s.renderQuery.Next(); {
		cur := s.renderQuery.Cursor()
		if fields := s.field.Slice(cur); fields != nil {
			if s.seenFields(s.base.Slice(cur), fields) {
				s.publish(b, cur, fields)
			}
		} else if !s.asleep.Present(cur) && !s.unfolded.Present(cur) && s.seenAny(cur.IDs) {
			s.publish(b, cur, nil)
		}
	}
}

// findUnfolded has unfoldedIn the stretches unfolded by whole and sector, as a Publish spreads
// its first field: one meeting none walks none
func (s *renderer) findUnfolded() {
	s.found = true
	s.unfoldedIn = s.unfoldedIn[:0]
	for s.stretches.All(); s.stretches.Next(); {
		for _, st := range s.stretch.Slice(s.stretches.Cursor()) {
			s.unfoldedIn = append(s.unfoldedIn, place{whole: st.Whole, sector: st.Sector})
		}
	}
	slices.SortFunc(s.unfoldedIn, byPlace)
}

// Swap has what was published last drawn.
func (s *renderer) Swap() {
	s.figures.Swap()
	s.aggregate.swap()
}

// publish settles the chunk under cur, some camera seeing it, fields its Fields in a chunk of
// fields
func (s *renderer) publish(b *figures, cur *goke.Cursor, fields []Field) {
	ids := cur.IDs
	for len(s.layers) < len(ids) {
		s.layers = append(s.layers, nil)
		s.shown = append(s.shown, false)
	}
	bases, appearances := s.base.Slice(cur), s.appearance.Slice(cur)
	zs, lies := s.z.Slice(cur), s.lies.Slice(cur)
	s.actives = s.active.Slice(cur)
	ruled := s.rules.ruled()
	if ruled {
		for i := range ids {
			s.layers[i] = append(s.layers[i][:0], appearances[i])
		}
		s.rules.Run(cur, s.layers[:len(ids)], s.shown[:len(ids)])
	}
	keen, wa := false, (*Atlas)(nil)
	if s.thinned || s.aggregate.summing {
		keen = s.keen.Slice(cur) != nil
		wa, _ = s.atlas.(*Atlas)
	}
	// a chunk of no fields, nothing thinned or summed, records what a camera sees: a test an entity
	plain := fields == nil && !s.thinned && !s.aggregate.summing
	for i, id := range ids {
		switch {
		case ruled && !s.shown[i]:
			continue
		case plain:
			if !s.sees(id) {
				continue
			}
		case fields != nil:
			if !s.spread(id, bases[i].Pos.AABB, fields[i].Size, wa, appearances[i].SpriteID) {
				continue
			}
		case s.aggregate.summing:
			if !s.sees(id) ||
				!s.recorded(id, bases[i].Pos.AABB, keen, wa, appearances[i].SpriteID) {
				continue
			}
		case !s.kept(id, bases[i].Pos.AABB, keen, wa, appearances[i].SpriteID):
			continue
		}
		n := len(b.all)
		b.all = slices.Grow(b.all, 1)[:n+1]
		e := &b.all[n]
		e.id, e.box = id, bases[i].Pos.AABB
		e.dir, e.speed = bases[i].Vel.Dir, bases[i].Vel.Value
		e.z, e.lie, e.piece, e.shadow = Z{}, 0, false, render.Shadowing{}
		if zs != nil {
			e.z = zs[i]
		}
		if lies != nil && lies[i].Length > 0 {
			b.lies = append(b.lies, lies[i])
			e.lie = int32(len(b.lies))
		}
		if s.lighting != nil {
			e.shadow = s.lighting.Mark(cur, i)
		}
		e.from = int32(len(b.layers))
		if !ruled {
			e.n = 1
			b.layers = append(b.layers, layer{Appearance: appearances[i],
				progress: s.progress(appearances[i].SpriteID, i)})
		} else {
			e.n = uint16(len(s.layers[i]))
			for _, l := range s.layers[i] {
				b.layers = append(b.layers,
					layer{Appearance: l, progress: s.progress(l.SpriteID, i)})
			}
		}
		if fields != nil {
			b.piecesOf(n, s.pieces)
		}
	}
}

// piecesOf lays the record n over the first of pieces, a copy of it over each of the others
func (b *figures) piecesOf(n int, pieces []plane.AABB) {
	e := &b.all[n]
	e.box, e.piece = pieces[0], true
	for _, p := range pieces[1:] {
		o := b.all[n]
		o.box = p
		b.all = append(b.all, o)
	}
}

// whether some camera sees any of ids, so the looks' rules run on a chunk worth drawing alone
func (s *renderer) seenAny(ids []uid.UID64) bool {
	for _, id := range ids {
		if s.sees(id) {
			return true
		}
	}
	return false
}

// whether some camera sees any of the fields standing over bases, as seenAny
func (s *renderer) seenFields(bases []Base, fields []Field) bool {
	for i := range fields {
		if s.seesField(bases[i].Pos.AABB, fields[i].Size) {
			return true
		}
	}
	return false
}

// seesField reports whether some camera's bounds meet the field standing over own, of size
func (s *renderer) seesField(own plane.AABB, size geom.Vec) bool {
	wide := fieldBox(own, size)
	for _, v := range s.seeing {
		if s.meets(v, wide.AABB) {
			return true
		}
	}
	return false
}

// spread has s.pieces the parts of the field id, standing over own and of size, one a sector it
// covers, that a view draws short of L3 or where its kind is not summed (wa's), and sums the
// others a view draws at L3 into the aggregate; it reports whether it has any. A part in a
// sector the field is unfolded in is neither, its parts drawn there; a field is never thinned
func (s *renderer) spread(id uid.UID64, own plane.AABB, size geom.Vec, wa *Atlas,
	sprite render.SpriteID) bool {
	c, summed := [4]uint8{}, false
	if s.aggregate.summing {
		c, summed = wa.blobColour(sprite)
	}
	if !s.found {
		s.findUnfolded()
	}
	s.pieces = s.pieces[:0]
	s.parts = s.coverOf(fieldBox(own, size).AABB, s.parts[:0])
	for _, p := range s.parts {
		if _, unfolded := slices.BinarySearchFunc(s.unfoldedIn,
			place{whole: id, sector: p.Sector}, byPlace); unfolded {
			continue
		}
		box := plane.NewAABB(p.Box.TopLeft, p.Box.BottomRight.X-p.Box.TopLeft.X,
			p.Box.BottomRight.Y-p.Box.TopLeft.Y)
		record, sum := false, false
		for _, v := range s.seeing {
			switch level, ok := s.pieceAt(v, box); {
			case !ok:
			case level == lod.L3 && summed:
				sum = true
			default:
				record = true
			}
		}
		if sum {
			s.aggregate.add(box, c)
		}
		if record {
			s.pieces = append(s.pieces, box)
		}
	}
	return len(s.pieces) > 0
}

// coverOf appends to dst the sectors box covers with its part in each; with no sectors one, 0,
// holding the whole of it
func (s *renderer) coverOf(box geom.AABB, dst []sector.Part) []sector.Part {
	if s.covers == nil {
		return append(dst, sector.Part{Box: box})
	}
	return s.covers(box, dst)
}

// pieceAt is the level v draws a field's part over box at, its sector's, false where v sees it
// not
func (s *renderer) pieceAt(v *view.View, box plane.AABB) (lod.Level, bool) {
	if !s.meets(v, box.AABB) {
		return lod.L4, false
	}
	mid := Position{AABB: box}.Center()
	level := v.Level(mid.X, mid.Y)
	return level, level != lod.L4
}

func byPlace(a, b place) int {
	return cmp.Or(cmp.Compare(a.whole, b.whole), cmp.Compare(a.sector, b.sector))
}

// keeping is the share of each level's entities a view keeps, as a bound on their identities
// stirred, zero for all; thins, whether it keeps less than all at some level, least the least
// bound but zero; l3, whether it sees a sector at L3, whose figures are summed into the ground
type keeping struct {
	under [lod.Levels]uint64
	least uint64
	thins bool
	l3    bool
}

// keepAll works out what each view published keeps of each level, by its ladder's Keep, and
// whether the figures some view sees at L3 are summed into the aggregate
func (s *renderer) keepAll() {
	s.keeps, s.thinned, s.aggregate.summing = s.keeps[:0], false, false
	wa, _ := s.atlas.(*Atlas)
	sums := s.aggregate.img != nil && wa != nil && len(wa.colours) > 0
	for _, v := range s.seeing {
		var k keeping
		if g := v.Grid(); g != nil {
			for l, share := range g.Ladder().Keep {
				if share > 0 && share < 1 {
					k.under[l] = uint64(float64(share) * 0x1p64)
					if !k.thins || k.under[l] < k.least {
						k.least = k.under[l]
					}
					k.thins = true
				}
			}
			if sums {
				v.Sectors(func(_ int, _ geom.AABB, l lod.Level) { k.l3 = k.l3 || l == lod.L3 })
			}
		}
		s.thinned = s.thinned || k.thins
		s.aggregate.summing = s.aggregate.summing || k.l3
		s.keeps = append(s.keeps, k)
	}
}

// recorded reports whether id, its box box and sprite sprite, is published as a record, and
// sums it into the aggregate where a view keeps it at L3 and its kind is summed there (wa's)
func (s *renderer) recorded(id uid.UID64, box plane.AABB, keen bool, wa *Atlas,
	sprite render.SpriteID) bool {
	c, ok := wa.blobColour(sprite)
	switch {
	case ok:
	case s.thinned:
		return s.kept(id, box, keen, wa, sprite)
	default:
		return s.sees(id)
	}
	record, sum := s.fate(id, box, keen, wa, sprite)
	if sum {
		s.aggregate.add(box, c)
	}
	return record
}

// fate is whether a view keeping id at a level short of L3 records it, and whether one keeping
// it at L3 sums it, its kind summed there
func (s *renderer) fate(id uid.UID64, box plane.AABB, keen bool, wa *Atlas,
	sprite render.SpriteID) (record, sum bool) {
	for k, v := range s.seeing {
		if !v.Contains(id) {
			continue
		}
		keep := &s.keeps[k]
		h, thins := uint64(0), false
		if keep.thins {
			h = stirred(id)
			thins = h >= keep.least && !keen && !wa.isAlways(sprite)
		}
		// a view seeing nothing at L3 draws id short of it, at whatever level
		level := lod.L0
		if keep.l3 || thins {
			level = s.level(v, box.AABB, Position{AABB: box}.Center())
		}
		if under := keep.under[level]; thins && under != 0 && h >= under &&
			!wa.declares(sprite, level) {
			continue
		}
		if level == lod.L3 {
			sum = true
		} else {
			record = true
		}
		if record && sum {
			break
		}
	}
	return record, sum
}

// kept reports whether a view seeing id keeps it at the level it draws box at: its identity
// stirred falls under the level's share, it is keen, or sprite is drawn in the place of a kind
// never thinned or in a look declared for that level (wa's)
func (s *renderer) kept(id uid.UID64, box plane.AABB, keen bool, wa *Atlas,
	sprite render.SpriteID) bool {
	for k, v := range s.seeing {
		if !v.Contains(id) {
			continue
		}
		keep := &s.keeps[k]
		h := stirred(id)
		if !keep.thins || h < keep.least || keen || wa != nil && wa.isAlways(sprite) {
			return true
		}
		level := s.level(v, box.AABB, Position{AABB: box}.Center())
		if under := keep.under[level]; under == 0 || h < under ||
			wa != nil && wa.declares(sprite, level) {
			return true
		}
	}
	return false
}

// stirred is id's bits mixed, so a share of the identities in any run of them is a share of the
// whole
func stirred(id uid.UID64) uint64 { return mixed(uint64(id)) }

func (s *renderer) sees(id uid.UID64) bool {
	for _, v := range s.seeing {
		if v.Contains(id) {
			return true
		}
	}
	return false
}

// how far the effect a material is under has run on the entity i of the chunk, 0 for none
func (s *renderer) progress(id render.SpriteID, i int) float32 {
	if len(s.phased) == 0 || s.actives == nil {
		return 0
	}
	ph, ok := s.phased[id]
	if !ok {
		return 0
	}
	left, on := s.actives[i].Left(ph.effect)
	return ph.progress(left, on)
}
