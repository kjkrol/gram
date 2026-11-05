package world

import (
	"cmp"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// figures are the entities the renderer draws, as the last Publish settled them: the clock as
// shown, what each camera saw, every entity some camera saw, what the sheets hold and the loose
// units gathered at the strategic scale.
type figures struct {
	shown   time.Duration
	clocked bool
	views   map[camera.Camera]*view.View
	all     []figure
	layers  []layer
	lies    []Lie
	sheet   sheetFrame
	far     farFrame
}

// figure is an entity drawn, a Field one a sector it covers: its box, the Field's part in that
// sector where piece, its heading and speed, its Z, its layers figures.layers[from:from+n], its
// Lie figures.lies[lie-1] (0 for none), its shadow as the Lighting says; sheet, one more than the
// sector whose sheet stands in its place from L4 on while it holds a photograph of it, 0 for
// none; fine, whether a view draws it short of L4 all the same; banner, whether a view draws it
// in its kind's banner where its kind declares a look; ringed, whether a view draws it, fixed by
// a seam, finer than its middle's sector
type figure struct {
	id        uid.UID64
	box       plane.AABB
	dir       geom.Vec
	speed     float32
	z         Z
	from, lie int32
	sheet     int32
	n         uint16
	piece     bool
	fine      bool
	banner    bool
	ringed    bool
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
		Optional(&s.unfolded).Optional(&s.fixed)
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
	s.farBegin(b)
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
	s.keepAll()
	s.sheet.recall(&b.sheet)
	if s.atlas == nil || s.renderQuery == nil || len(s.seeing) == 0 {
		b.sheet.on = false
		return
	}
	sheeting := s.sheet.begin(&b.sheet, s.seeing)
	s.found = false
	// what lies asleep at L6 is drawn by no camera, nor a point aggregate unfolded, its parts
	// alone; a field is, wherever its box is; what stands where a sheet is drawn is photographed
	// whether a camera sees it or not
	for s.renderQuery.All(); s.renderQuery.Next(); {
		cur := s.renderQuery.Cursor()
		if fields := s.field.Slice(cur); fields != nil {
			if s.seenFields(s.base.Slice(cur), fields) {
				s.publish(b, cur, fields, false)
			}
		} else if !s.asleep.Present(cur) && !s.unfolded.Present(cur) {
			fixed := sheeting && s.fixed.Present(cur)
			if s.seenAny(cur.IDs) || fixed && s.wantedAny(s.base.Slice(cur)) {
				s.publish(b, cur, nil, fixed)
			}
		}
	}
	s.farSettle()
	if sheeting {
		s.sheet.settle(&b.sheet)
		if s.sheet.droppable > 0 {
			b.drop()
		}
	}
}

// wantedAny reports whether any of bases stands where a view draws a sheet
func (s *renderer) wantedAny(bases []Base) bool {
	for i := range bases {
		if s.sheet.wants(bases[i].Pos.AABB.AABB) {
			return true
		}
	}
	return false
}

// drop leaves out the records of what a sheet holding a photograph of it stands in the place of
// wherever it is drawn
func (b *figures) drop() {
	held, kept := b.sheet.held, b.all[:0]
	for k := range b.all {
		if e := &b.all[k]; e.sheet == 0 || e.fine || !held[e.sheet-1] {
			kept = append(kept, *e)
		}
	}
	b.all = kept
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

// Swap has what was published last drawn, its photographs of what stands developed as a draw
// first reads a sheet: before the ground, which a frame draws before the world's Compose.
func (s *renderer) Swap() {
	s.figures.Swap()
	if len(s.figures.Front().sheet.photos) > 0 {
		s.sheet.schedule(s.developing)
	}
}

// publish settles the chunk under cur, some camera seeing it or, of things fixed, a sheet drawn
// where it stands, fields its Fields in a chunk of fields
func (s *renderer) publish(b *figures, cur *goke.Cursor, fields []Field, fixed bool) {
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
	if s.thinned {
		keen = s.keen.Slice(cur) != nil
		wa, _ = s.atlas.(*Atlas)
	}
	// what a sheet or a banner may stand in for is walked apart, so the walk of the rest holds no
	// more live than it reads
	if fixed || (s.far.banners || s.far.counts) && fields == nil && !s.fixed.Present(cur) {
		s.publishStandIns(b, cur, fixed, keen, wa)
		return
	}
	// a chunk of no fields, nothing thinned, records what a camera sees: a test an entity
	plain := fields == nil && !s.thinned
	for i, id := range ids {
		switch {
		case ruled && !s.shown[i]:
			continue
		case plain:
			if !s.sees(id) {
				continue
			}
		case fields != nil:
			if !s.spread(id, bases[i].Pos.AABB, fields[i].Size) {
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
		var z Z
		if zs != nil {
			z = zs[i]
		}
		// in the order they lie, so the zeros are stored together
		e.z, e.lie, e.sheet = z, 0, 0
		e.piece, e.fine, e.banner, e.ringed = false, false, false, false
		e.shadow = render.Shadowing{}
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
			if len(s.sheet.folded) > 0 {
				s.foldedPieces(b, n)
			}
		}
	}
}

// publishStandIns is publish of a chunk of no fields whose records a sheet or a banner may stand
// in for: its things fixed where fixed, else its loose units, the banners or the counts drawn
func (s *renderer) publishStandIns(b *figures, cur *goke.Cursor, fixed, keen bool, wa *Atlas) {
	ids := cur.IDs
	bases, appearances := s.base.Slice(cur), s.appearance.Slice(cur)
	zs, lies := s.z.Slice(cur), s.lies.Slice(cur)
	ruled := s.rules.ruled()
	ws, _ := s.atlas.(*Atlas)
	plain := !s.thinned
	for i, id := range ids {
		if ruled && !s.shown[i] {
			continue
		}
		var z Z
		if zs != nil {
			z = zs[i]
		}
		sheet, alive, ringed := int32(0), false, false
		// where no view draws a sheet, what stands is neither photographed nor left to one; what a
		// view draws alive by a seam is drawn whole there, never thinned, and no sheet holds it
		if fixed && s.sheet.wants(bases[i].Pos.AABB.AABB) {
			if alive, ringed = s.alive(id, bases[i].Pos.AABB); !alive {
				var lie *Lie
				if lies != nil && lies[i].Length > 0 {
					lie = &lies[i]
				}
				sheet = s.stand(id, &bases[i], z, lie, s.first(i, appearances), ws)
			}
		}
		bannered := false
		if !fixed {
			var own bool
			if own, bannered = s.farOf(id, &bases[i], z, s.first(i, appearances), ws); !own {
				continue
			}
		}
		switch {
		case plain:
			if !s.sees(id) {
				continue
			}
		case !alive && !s.kept(id, bases[i].Pos.AABB, keen, wa, appearances[i].SpriteID):
			continue
		}
		n := len(b.all)
		b.all = slices.Grow(b.all, 1)[:n+1]
		e := &b.all[n]
		e.id, e.box = id, bases[i].Pos.AABB
		e.dir, e.speed = bases[i].Vel.Dir, bases[i].Vel.Value
		e.z, e.lie, e.sheet = z, 0, sheet
		e.piece, e.fine, e.banner, e.ringed = false, false, bannered, ringed
		e.shadow = render.Shadowing{}
		if sheet > 0 {
			if e.fine = s.fine(id, e.box); !e.fine {
				s.sheet.droppable++
			}
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
	}
}

// first is the sprite of the first layer of the entity i of the chunk published, appearances its
// column
func (s *renderer) first(i int, appearances []render.Appearance) render.SpriteID {
	if s.rules.ruled() {
		return s.layers[i][0].SpriteID
	}
	return appearances[i].SpriteID
}

// stand photographs into the sheets the thing fixed id standing over base at z, lying as lie
// says where it is not nil, in its first layer's look sprite as it is drawn at L3, wa's; it is
// one more than the sector of its middle where its kind declares no look of its own at L4 and it
// is photographed so, else 0
func (s *renderer) stand(id uid.UID64, base *Base, z Z, lie *Lie, sprite render.SpriteID,
	wa *Atlas) int32 {
	var levels byLevel
	if wa != nil {
		if wa.declares(sprite, lod.Strategic) {
			return 0
		}
		levels = wa.levels
	}
	l3, ok := levels.at(sprite, lod.L3)
	if !ok || !s.photographable(wa, l3) {
		return 0
	}
	box := base.Pos.AABB
	var at shot
	s.photographed(&at, box, base.Vel.Dir, lie, z, l3)
	s.sheet.stand(box.AABB, signed(uint64(id), l3, z.Top(), box.TopLeft), &at)
	mid := Position{AABB: box}.Center()
	return int32(s.sheet.grid.Of(mid.X, mid.Y)) + 1
}

// alive reports whether a view seeing id draws the thing fixed over box short of L4 where it draws
// a sector the box lies in at L4 or coarser, so that view draws it alive and no sheet holds it,
// and ringed, whether a view draws it finer than its middle's sector, by a seam
func (s *renderer) alive(id uid.UID64, box plane.AABB) (alive, ringed bool) {
	mid := Position{AABB: box}.Center()
	crosses, near := s.sheet.seams(box.AABB, mid)
	if !near {
		return false, false
	}
	for _, v := range s.seeing {
		if !v.Contains(id) {
			continue
		}
		level := s.level(v, box.AABB, mid)
		switch drawn := s.drawn(v, box.AABB, level, &s.reach); {
		case drawn < level:
			return true, true
		case drawn < lod.Strategic && crosses && !alive:
			alive = s.sheetedIn(v, box.AABB)
		}
	}
	return alive, false
}

// drawn is the level v draws a thing fixed over box at whose middle's sector it draws at level:
// from L4 out, the finest short of L4 of the sectors the box grown by its longer side covers
// where there is one, so a thing by a seam is drawn whole by its finer side; parts, the scratch
func (s *renderer) drawn(v *view.View, box geom.AABB, level lod.Level,
	parts *[]sector.Part) lod.Level {
	if level < lod.Strategic || level == lod.Omitted {
		return level
	}
	tl, br := box.TopLeft, box.BottomRight
	by := max(br.X-tl.X, br.Y-tl.Y)
	grown := geom.NewAABB(geom.NewVec(tl.X-by, tl.Y-by), geom.NewVec(br.X+by, br.Y+by))
	*parts = s.coverOf(grown, (*parts)[:0])
	finest := level
	for _, p := range *parts {
		finest = min(finest, levelOver(v, p.Box))
	}
	if finest < lod.Strategic {
		return finest
	}
	return level
}

// sheetedIn reports whether v draws at L4 or coarser, a sheet there, a sector box lies in
func (s *renderer) sheetedIn(v *view.View, box geom.AABB) bool {
	s.reach = s.coverOf(box, s.reach[:0])
	for _, p := range s.reach {
		if l := levelOver(v, p.Box); l >= lod.Strategic && l < lod.Omitted {
			return true
		}
	}
	return false
}

// levelOver is the level v draws the sector at, b a part of it
func levelOver(v *view.View, b geom.AABB) lod.Level {
	return v.Level((b.TopLeft.X+b.BottomRight.X)/2, (b.TopLeft.Y+b.BottomRight.Y)/2)
}

// photographed lays at as the sheets photograph a thing over box headed dir at z in the sprite
// id, over its Lie where lie is not nil
func (s *renderer) photographed(at *shot, box plane.AABB, dir geom.Vec, lie *Lie, z Z,
	id render.SpriteID) {
	mid := Position{AABB: box}.Center()
	along, across, angle, flat := box.Size.X/2, box.Size.Y/2, float32(0), s.lying[id]
	if lie != nil {
		if dir == (geom.Vec{}) {
			dir = geom.NewVec(1, 0)
		}
		mid = geom.NewVec(mid.X+dir.X*lie.Ahead, mid.Y+dir.Y*lie.Ahead)
		along, across, angle, flat = lie.Length/2, lie.Width/2, f32.Atan2(dir.Y, dir.X), true
	}
	lies := float32(0)
	if flat {
		lies = 1
	}
	x0, y0, x1, y1 := s.atlas.UV(id)
	*at = shot{mid.X, mid.Y, along, across, x0, y0, x1, y1, z.Altitude, z.Height, lies, angle}
}

// photographable reports whether the sprite id has a look of its own on the atlas, wa where it
// is the world's, that is no material: a sprite of a kind given no look is drawn at no level
func (s *renderer) photographable(wa *Atlas, id render.SpriteID) bool {
	if int(id) < len(s.materials) && s.materials[id] {
		return false
	}
	return wa == nil || int(id) < len(wa.levels) && !wa.levels[id][lod.Omitted].drawn
}

// fine reports whether a view seeing id draws it over box short of L4
func (s *renderer) fine(id uid.UID64, box plane.AABB) bool {
	mid := Position{AABB: box}.Center()
	for _, v := range s.seeing {
		if v.Contains(id) && s.level(v, box.AABB, mid) < lod.Strategic {
			return true
		}
	}
	return false
}

// foldedPieces has the pieces of the field recorded from n on whose stretches the sheets keep
// the parts of stand in the place of the field while their sectors hold a photograph of them
func (s *renderer) foldedPieces(b *figures, n int) {
	for k := n; k < len(b.all); k++ {
		e := &b.all[k]
		mid := Position{AABB: e.box}.Center()
		i := s.sheet.grid.Of(mid.X, mid.Y)
		if _, ok := s.sheet.folded[place{whole: e.id, sector: i}]; ok {
			e.sheet, e.fine = int32(i)+1, false
			s.sheet.droppable++
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

// whether some camera sees any of ids, so the looks' rules run on a chunk worth drawing alone;
// out of line, so what its caller holds live spills nothing into its walk
//
//go:noinline
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
// covers, that a view draws; it reports whether it has any. A part in a sector the field is
// unfolded in is none, its parts drawn there; a field is never thinned
func (s *renderer) spread(id uid.UID64, own plane.AABB, size geom.Vec) bool {
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
		for _, v := range s.seeing {
			if _, ok := s.pieceAt(v, box); ok {
				s.pieces = append(s.pieces, box)
				break
			}
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
		return lod.Omitted, false
	}
	mid := Position{AABB: box}.Center()
	level := v.Level(mid.X, mid.Y)
	return level, level != lod.Omitted
}

func byPlace(a, b place) int {
	return cmp.Or(cmp.Compare(a.whole, b.whole), cmp.Compare(a.sector, b.sector))
}

// keeping is the share of each level's entities a view keeps, as a bound on their identities
// stirred, zero for all; thins, whether it keeps less than all at some level, least the least
// bound but zero
type keeping struct {
	under [lod.Levels]uint64
	least uint64
	thins bool
}

// keepAll works out what each view published keeps of each level, by its ladder's Keep
func (s *renderer) keepAll() {
	s.keeps, s.thinned = s.keeps[:0], false
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
		}
		s.thinned = s.thinned || k.thins
		s.keeps = append(s.keeps, k)
	}
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

// mixed is x's bits mixed (splitmix64's finaliser), so a share of any run of numbers is a share of
// the whole
func mixed(x uint64) uint64 {
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}

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
	return s.phaseOf(id, i)
}

// phaseOf is progress where a material is phased and the chunk is under effects, apart so that
// progress's test inlines
func (s *renderer) phaseOf(id render.SpriteID, i int) float32 {
	ph, ok := s.phased[id]
	if !ok {
		return 0
	}
	left, on := s.actives[i].Left(ph.effect)
	return ph.progress(left, on)
}
