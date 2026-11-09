package world

import (
	"image/color"
	"strconv"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// BannerStyle is how a kind's banner is drawn: a badge of Shape at least Size shown pixels across,
// in Fill within a rim in Rim, each zero its default (a disc 14 across in the kind's far colour,
// its rim darker). Declared on the world's atlas (Slot.Styled), it has the kind drawn in banners at
// the strategic scale even without a look declared there.
type BannerStyle struct {
	Shape render.BadgeShape
	Size  float32
	Fill  color.RGBA
	Rim   color.RGBA
}

// Style: a figure declares it on the world's atlas (render.Style).
func (BannerStyle) Style() {}

var _ render.Style = BannerStyle{}

// shown pixels: how wide a banner is unless its style says
const bannerSize = 14

// filled is st with its zeros the defaults, colour the kind's far colour
func (st BannerStyle) filled(colour [4]uint8) BannerStyle {
	if st.Size <= 0 {
		st.Size = bannerSize
	}
	if st.Fill.A == 0 {
		st.Fill = color.RGBA{R: colour[0], G: colour[1], B: colour[2], A: 255}
	}
	if st.Rim.A == 0 {
		st.Rim = color.RGBA{R: st.Fill.R / 3, G: st.Fill.G / 3, B: st.Fill.B / 3, A: 255}
	}
	return st
}

// farFrame is what a frame draws of the loose units at the strategic scale: a kind's units a
// view draws in a sector gathered, ids[from:from+count] each, and whether their counts are drawn
type farFrame struct {
	groups []farGroup
	ids    []uid.UID64
	counts bool
}

// farGroup is a kind's loose units a view draws at level in a sector: one banner, a badge in style
// width across, or points counted alone; drawn over a box of their mean size at their centroid, at
// their mean Z, in colour; count is the entities, heads the units they stand for
type farGroup struct {
	view        *view.View
	kind        kind.ID
	level       lod.Level
	banner      bool
	sector      int32
	sprite      render.SpriteID
	colour      [4]uint8
	style       BannerStyle
	width       float32
	from, count int32
	heads       int32
	centre      geom.Vec
	size        geom.Vec
	z           Z
}

// farring is how a Publish gathers the loose units at the strategic scale: whether they go in
// banners and are counted, as look and counted say (nil, neither), the group of each key, the
// sums of each group and the members met, by group; labels are the counts as text
type farring struct {
	look    func() FarLook
	counted func() bool
	banners bool
	counts  bool
	frame   *farFrame
	byKey   map[farKey]int32
	last    farKey
	lastAt  int32
	// by group its key and the one it is joined into, nearer the first; the group whose
	// neighbours are sought, farBeside bound once, and whether any joined
	keys    []farKey
	root    []int32
	near    int32
	beside  func(j int)
	joined  bool
	sums    []farSum
	members []farMember
	at      []int32
	labels  []string
}

// farKey is a group's place: the view published, the sector, the kind and the level drawn at
type farKey struct {
	view   int32
	sector int32
	kind   kind.ID
	level  lod.Level
}

// farSum is a group's middles, sizes and Zs summed as the walk meets its units, and the corners
// of the box round their boxes
type farSum struct {
	x, y, w, h, altitude, height float64
	lo, hi                       geom.Vec
}

type farMember struct {
	group int32
	id    uid.UID64
}

// farBegin readies b to gather the loose units at the strategic scale anew, as the settings say
// now; it reports whether any are gathered
func (s *renderer) farBegin(b *figures) bool {
	f := &s.far
	b.far.groups, b.far.ids = b.far.groups[:0], b.far.ids[:0]
	f.banners = f.look != nil && f.look() == Banners
	f.counts = f.counted != nil && f.counted()
	b.far.counts = f.counts
	f.frame, f.lastAt = &b.far, -1
	f.sums, f.members, f.keys = f.sums[:0], f.members[:0], f.keys[:0]
	clear(f.byKey)
	return f.banners || f.counts
}

// farOf gathers id, standing over base at z in sprite, its first layer's, for heads units, into
// its kind's group in its sector for each view drawing it from L4 on, in a banner where the
// banners are drawn and its kind declares a look there (wa's), else counted where the counts are
// drawn; it reports whether some view draws it as itself, and whether one draws it in a banner
func (s *renderer) farOf(id uid.UID64, base *Base, z Z, sprite render.SpriteID, wa *Atlas,
	heads int32) (own, bannered bool) {
	box := base.Pos.AABB
	mid := Position{AABB: box}.Center()
	for k, v := range s.seeing {
		if !v.Contains(id) {
			continue
		}
		level := s.level(v, box.AABB, mid)
		if level < lod.Strategic || level >= lod.Omitted || v.Grid() == nil {
			own = true
			continue
		}
		banner := s.far.banners && wa != nil && wa.bannered(sprite, level)
		if banner {
			bannered = true
		} else {
			own = true
		}
		if banner || s.far.counts {
			key := farKey{view: int32(k), sector: int32(v.Grid().Of(mid.X, mid.Y)),
				kind: base.TypeID, level: level}
			s.join(key, v, banner, box, z, sprite, wa, id, heads)
		}
	}
	return own, bannered
}

// join adds id, standing over box at z in sprite for heads units, to the group of key, v's, made
// at its first
func (s *renderer) join(key farKey, v *view.View, banner bool, box plane.AABB, z Z,
	sprite render.SpriteID, wa *Atlas, id uid.UID64, heads int32) {
	f := &s.far
	g := f.lastAt
	if g < 0 || key != f.last {
		var ok bool
		if g, ok = f.byKey[key]; !ok {
			g = int32(len(f.frame.groups))
			if f.byKey == nil {
				f.byKey = map[farKey]int32{}
			}
			f.byKey[key] = g
			colour := [4]uint8{255, 255, 255, 255}
			if wa != nil {
				if c, ok := wa.farColour(sprite); ok {
					colour = c
				}
			}
			f.frame.groups = append(f.frame.groups, farGroup{view: v, kind: key.kind,
				level: key.level, banner: banner, sector: key.sector, sprite: sprite,
				colour: colour, style: wa.bannerStyle(sprite, colour)})
			f.sums = append(f.sums, farSum{lo: box.TopLeft, hi: box.TopLeft.Add(box.Size)})
			f.keys = append(f.keys, key)
		}
		f.last, f.lastAt = key, g
	}
	f.frame.groups[g].count++
	f.frame.groups[g].heads += heads
	mid := Position{AABB: box}.Center()
	sum := &f.sums[g]
	far := box.TopLeft.Add(box.Size)
	sum.lo = geom.NewVec(min(sum.lo.X, box.TopLeft.X), min(sum.lo.Y, box.TopLeft.Y))
	sum.hi = geom.NewVec(max(sum.hi.X, far.X), max(sum.hi.Y, far.Y))
	sum.x += float64(mid.X)
	sum.y += float64(mid.Y)
	sum.w += float64(box.Size.X)
	sum.h += float64(box.Size.Y)
	sum.altitude += float64(z.Altitude)
	sum.height += float64(z.Height)
	f.members = append(f.members, farMember{group: g, id: id})
}

// farSettle lays the groups the walk gathered out for the frame, those of a crowd across a seam
// joined: each one's units in a run of ids in the order met, its centroid, mean size and mean Z
func (s *renderer) farSettle() {
	f := &s.far
	b := f.frame
	if b == nil || len(b.groups) == 0 {
		return
	}
	joined := s.farJoin()
	f.at = f.at[:0]
	from := int32(0)
	for k := range b.groups {
		g, sum := &b.groups[k], &f.sums[k]
		g.from = from
		f.at = append(f.at, from)
		from += g.count
		n := float64(g.count)
		g.centre = geom.NewVec(float32(sum.x/n), float32(sum.y/n))
		g.size = geom.NewVec(float32(sum.w/n), float32(sum.h/n))
		g.z = Z{Altitude: float32(sum.altitude / n), Height: float32(sum.height / n)}
		if joined {
			s.farWrap(g)
		}
		if g.banner {
			g.width = render.BadgeWidth(g.numbered(b.counts), g.style.Shape, g.style.Size)
		}
	}
	if cap(b.ids) < int(from) {
		b.ids = make([]uid.UID64, from)
	}
	b.ids = b.ids[:from]
	for _, m := range f.members {
		g := m.group
		if joined {
			g = f.root[g]
		}
		b.ids[f.at[g]] = m.id
		f.at[g]++
	}
}

// farJoin joins the groups of one view, kind and level, banners both or neither, in neighbouring
// sectors whose units' boxes lie within the world's Small of each other, the shorter way round
// where it wraps, each into the first of its crowd; it reports whether any joined, root then by
// group the place of the one it is joined into among those left
func (s *renderer) farJoin() bool {
	f, grid := &s.far, s.sheet.grid
	groups := f.frame.groups
	if grid == nil || len(groups) < 2 {
		return false
	}
	f.root = f.root[:0]
	for k := range groups {
		f.root = append(f.root, int32(k))
	}
	if f.beside == nil {
		f.beside = s.farBeside
	}
	f.joined = false
	for k := range groups {
		f.near = int32(k)
		grid.Neighbours(int(groups[k].sector), f.beside)
	}
	if !f.joined {
		return false
	}
	// a group's root lies before it, so the root's place is known as the group is met
	n := int32(0)
	for k := range groups {
		if r := f.root[k]; r != int32(k) {
			f.root[k] = f.root[r]
			s.farInto(&groups[f.root[k]], &f.sums[f.root[k]], &groups[k], &f.sums[k])
			continue
		}
		f.root[k] = n
		groups[n], f.sums[n] = groups[k], f.sums[k]
		n++
	}
	f.frame.groups, f.sums = groups[:n], f.sums[:n]
	return true
}

// farBeside joins the group whose neighbours are sought and that of its view, kind and level in
// sector j, met from the first of the two, where they lie close
func (s *renderer) farBeside(j int) {
	f := &s.far
	k, key := f.near, f.keys[f.near]
	key.sector = int32(j)
	h, ok := f.byKey[key]
	groups := f.frame.groups
	if !ok || h < k || groups[h].banner != groups[k].banner || !s.farClose(&f.sums[k], &f.sums[h]) {
		return
	}
	a, b := f.find(k), f.find(h)
	if a > b {
		a, b = b, a
	}
	f.root[b] = a
	f.joined = f.joined || a != b
}

// find is the first group of k's crowd, the way to it halved
func (f *farring) find(k int32) int32 {
	for f.root[k] != k {
		f.root[k] = f.root[f.root[k]]
		k = f.root[k]
	}
	return k
}

// farClose reports whether the boxes round the units of two groups lie within the world's Small
// of each other along both axes
func (s *renderer) farClose(a, b *farSum) bool {
	small, sp := s.sheet.small, s.space
	return farGap(a.lo.X, a.hi.X, b.lo.X, b.hi.X, float32(sp.Width), sp.Edges.WrapsX()) <= small &&
		farGap(a.lo.Y, a.hi.Y, b.lo.Y, b.hi.Y, float32(sp.Height), sp.Edges.WrapsY()) <= small
}

// farGap is how far [alo, ahi] and [blo, bhi] lie apart on an axis size long, the shorter way
// round where it wraps; zero where they overlap
func farGap(alo, ahi, blo, bhi, size float32, wraps bool) float32 {
	d := max(blo-ahi, alo-bhi, 0)
	if wraps {
		d = min(d, max(blo+size-ahi, alo-bhi-size, 0), max(blo-size-ahi, alo-bhi+size, 0))
	}
	return d
}

// farInto adds the group g, its sums sum, to the group into, its sums to, g's middles taken the
// shorter way round to into's where the world wraps
func (s *renderer) farInto(into *farGroup, to *farSum, g *farGroup, sum *farSum) {
	var dx, dy float64
	if sp := s.space; sp.Edges.WrapsX() || sp.Edges.WrapsY() {
		n, m := float64(into.count), float64(g.count)
		dx = farShift(sum.x/m-to.x/n, float64(sp.Width), sp.Edges.WrapsX())
		dy = farShift(sum.y/m-to.y/n, float64(sp.Height), sp.Edges.WrapsY())
	}
	into.count += g.count
	into.heads += g.heads
	to.x += sum.x + dx*float64(g.count)
	to.y += sum.y + dy*float64(g.count)
	to.w += sum.w
	to.h += sum.h
	to.altitude += sum.altitude
	to.height += sum.height
}

// farShift is the whole turns of an axis size long to add to a way d the shorter way round,
// where it wraps
func farShift(d, size float64, wraps bool) float64 {
	switch {
	case !wraps:
		return 0
	case d > size/2:
		return -size
	case d < -size/2:
		return size
	}
	return 0
}

// farWrap has the centre of g, a group maybe joined across a seam, within the world where it
// wraps, and its sector the one holding it
func (s *renderer) farWrap(g *farGroup) {
	sp := s.space
	if sp.Edges.WrapsX() {
		g.centre.X = farWrapped(g.centre.X, float32(sp.Width))
	}
	if sp.Edges.WrapsY() {
		g.centre.Y = farWrapped(g.centre.Y, float32(sp.Height))
	}
	g.sector = int32(s.sheet.grid.Of(g.centre.X, g.centre.Y))
}

// farWrapped is x within an axis size long that wraps
func farWrapped(x, size float32) float32 {
	if x = f32.Mod(x, size); x < 0 {
		x += size
	}
	return x
}

// box is the box g is drawn over
func (g *farGroup) box() plane.AABB {
	return plane.NewAABB(geom.NewVec(g.centre.X-g.size.X/2, g.centre.Y-g.size.Y/2), g.size.X,
		g.size.Y)
}

// numbered is the number g's badge shows, counts saying whether the counts are drawn: its heads
// where they are two or more, else none
func (g *farGroup) numbered(counts bool) int {
	if !counts || g.heads < 2 {
		return 0
	}
	return int(g.heads)
}

// banner is the Banner of g, its units b's
func (b *farFrame) banner(g *farGroup) Banner {
	return Banner{Kind: g.kind, Sector: int(g.sector), Center: g.centre,
		IDs: b.ids[g.from : g.from+g.count : g.from+g.count], Count: int(g.heads),
		Box: g.box().AABB, Z: g.z, Reach: g.width / 2}
}

// bannersOf appends to dst the banners cam drew in the frame drawn
func (s *renderer) bannersOf(cam camera.Camera, dst []Banner) []Banner {
	figs := s.figures.Front()
	v := figs.views[camera.OriginOf(cam)]
	if v == nil {
		return dst
	}
	for k := range figs.far.groups {
		if g := &figs.far.groups[k]; g.view == v && g.banner {
			dst = append(dst, figs.far.banner(g))
		}
	}
	return dst
}

// bannerAt reports whether a record of a unit in a banner, its first layer drawn in the sprite
// id, is drawn as the banner by a view seeing it at level, wa's
func bannerAt(wa *Atlas, id render.SpriteID, level lod.Level) bool {
	return level >= lod.Strategic && level < lod.Omitted && wa != nil && wa.bannered(id, level)
}

// drawBanners has the labels show the banners of the groups v drew, each a badge in its kind's
// style standing over where the top of its box shows, holding its count where the counts are drawn
// and it counts two or more; it notes the lowest of them as Compose does and reports whether it
// drew any
func (s *renderer) drawBanners(f *render.Frame, figs *figures, v *view.View, sorts bool,
	lowest *render.At, low *float32, any *bool) bool {
	drew := false
	for k := range figs.far.groups {
		g := &figs.far.groups[k]
		if g.view != v || !g.banner {
			continue
		}
		if !*any || sorts || g.z.Altitude < *low {
			at := f.Lifted(g.centre.X, g.centre.Y, g.z.Altitude, FigureLift)
			if !*any || at.Before(*lowest) {
				*lowest, *low, *any = at, g.z.Altitude, true
			}
		}
		// a badge round where its units stand would sink into the ground before them
		top, st := g.z.Top(), &g.style
		s.labels.Badge(g.numbered(figs.far.counts), st.Shape, g.centre.X, g.centre.Y, top,
			top+FigureLift, 0, -badgeLift(g.width/2), g.width, st.Fill, st.Rim)
		drew = true
	}
	return drew
}

// pixels: how far over the top of what it counts a count or a badge stands
const countAbove = 2

// badgeLift is how many shown pixels over where the top of its units shows the middle of a badge
// reaching reach round it stands, on a whole pixel
func badgeLift(reach float32) float32 { return countAbove + f32.Ceil(reach) }

// most counts kept as text
const countsKept = 1 << 12

// drawCounts has the labels show, where the counts are drawn, the number of units each kind's
// points in a sector v drew stand for, two or more, by their centroid, in their kind's colour on a
// black shadow, as deep as they lie; it notes the lowest of them as Compose does and reports
// whether it drew any
func (s *renderer) drawCounts(f *render.Frame, figs *figures, v *view.View, sorts bool,
	lowest *render.At, low *float32, any *bool) bool {
	if !figs.far.counts {
		return false
	}
	font := render.DefaultFont()
	drew := false
	for k := range figs.far.groups {
		g := &figs.far.groups[k]
		if g.view != v || g.banner || g.heads < 2 {
			continue
		}
		if !*any || sorts || g.z.Altitude < *low {
			at := f.Lifted(g.centre.X, g.centre.Y, g.z.Altitude, FigureLift)
			if !*any || at.Before(*lowest) {
				*lowest, *low, *any = at, g.z.Altitude, true
			}
		}
		top, over := g.z.Top(), g.centre.Y
		label := s.countOf(g.heads)
		w, h := font.Measure(label)
		dx, dy := -w/2, -h-countAbove
		deep := top + FigureLift
		c := g.colour
		s.labels.Text(font, label, g.centre.X, over, top, deep, dx+1, dy+1, color.RGBA{A: 255})
		s.labels.Text(font, label, g.centre.X, over, top, deep, dx, dy,
			color.RGBA{R: c[0], G: c[1], B: c[2], A: 255})
		drew = true
	}
	return drew
}

// countOf is n as text, kept for the next frame where it is small
func (s *renderer) countOf(n int32) string {
	if n >= countsKept {
		return strconv.Itoa(int(n))
	}
	labels := &s.far.labels
	for int(n) >= len(*labels) {
		*labels = append(*labels, "")
	}
	if (*labels)[n] == "" {
		(*labels)[n] = strconv.Itoa(int(n))
	}
	return (*labels)[n]
}
