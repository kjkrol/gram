package world

import (
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

// figure is an entity drawn: its box, its heading and speed, its Z, its layers
// figures.layers[from:from+n], its Lie figures.lies[lie-1] (0 for none), its shadow as the
// Lighting says
type figure struct {
	id        uid.UID64
	box       plane.AABB
	dir       geom.Vec
	speed     float32
	z         Z
	from, lie int32
	n         uint16
	shadow    render.Shadowing
}

// layer is a sprite an entity is drawn with and how far the effect its material is under has run
type layer struct {
	render.Appearance
	progress float32
}

func (s *renderer) Init(si *goke.SysInit) {
	// what lies asleep at L4 is drawn by no camera
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z).Optional(&s.lies).
		Optional(&s.active).Optional(&s.keen).Exclude(goke.Exclude[sector.L4]())
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
	for s.renderQuery.All(); s.renderQuery.Next(); {
		s.publish(b, s.renderQuery.Cursor())
	}
}

// Swap has what was published last drawn.
func (s *renderer) Swap() {
	s.figures.Swap()
	s.aggregate.swap()
}

func (s *renderer) publish(b *figures, cur *goke.Cursor) {
	ids := cur.IDs
	for len(s.layers) < len(ids) {
		s.layers = append(s.layers, nil)
		s.shown = append(s.shown, false)
	}
	if !s.seenAny(ids) {
		return
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
	for i, id := range ids {
		switch {
		case ruled && !s.shown[i]:
			continue
		case s.aggregate.summing:
			if !s.sees(id) ||
				!s.recorded(id, bases[i].Pos.AABB, keen, wa, appearances[i].SpriteID) {
				continue
			}
		case !s.thinned:
			if !s.sees(id) {
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
		e.z, e.lie, e.shadow = Z{}, 0, render.Shadowing{}
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
			continue
		}
		e.n = uint16(len(s.layers[i]))
		for _, l := range s.layers[i] {
			b.layers = append(b.layers, layer{Appearance: l, progress: s.progress(l.SpriteID, i)})
		}
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
