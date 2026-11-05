package world

import (
	"image/color"
	"strconv"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ render.Glass = (*renderer)(nil)

// farFrame is what a frame draws of the loose units at the strategic scale: a kind's units a
// view draws in a sector gathered, ids[from:from+count] each, and whether their counts are drawn
type farFrame struct {
	groups []farGroup
	ids    []uid.UID64
	counts bool
}

// farGroup is a kind's loose units a view draws at level in a sector: one banner in the look of
// the first one's sprite there, or points counted alone; drawn over a box of their mean size at
// their centroid, at their mean Z, in colour
type farGroup struct {
	view        *view.View
	kind        kind.ID
	level       lod.Level
	banner      bool
	sector      int32
	sprite      render.SpriteID
	colour      [4]uint8
	from, count int32
	centre      geom.Vec
	size        geom.Vec
	z           Z
}

// farring is how a Publish gathers the loose units at the strategic scale: whether they go in
// banners and are counted, as look and counted say (nil, neither), the group of each key, the
// sums of each group and the members met, by group; labels are the counts as text, the glass's
type farring struct {
	look    func() FarLook
	counted func() bool
	banners bool
	counts  bool
	frame   *farFrame
	byKey   map[farKey]int32
	last    farKey
	lastAt  int32
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

// farSum is a group's middles, sizes and Zs summed as the walk meets its units
type farSum struct{ x, y, w, h, altitude, height float64 }

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
	f.sums, f.members = f.sums[:0], f.members[:0]
	clear(f.byKey)
	return f.banners || f.counts
}

// farOf gathers id, standing over base at z in sprite, its first layer's, into its kind's group
// in its sector for each view drawing it from L4 on, in a banner where the banners are drawn and
// its kind declares a look there (wa's), else counted where the counts are drawn; it reports
// whether some view draws it as itself, and whether one draws it in a banner
func (s *renderer) farOf(id uid.UID64, base *Base, z Z, sprite render.SpriteID,
	wa *Atlas) (own, bannered bool) {
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
		banner := s.far.banners && wa != nil && wa.declares(sprite, level)
		if banner {
			bannered = true
		} else {
			own = true
		}
		if banner || s.far.counts {
			key := farKey{view: int32(k), sector: int32(v.Grid().Of(mid.X, mid.Y)),
				kind: base.TypeID, level: level}
			s.join(key, v, banner, box, z, sprite, wa, id)
		}
	}
	return own, bannered
}

// join adds id, standing over box at z in sprite, to the group of key, v's, made at its first
func (s *renderer) join(key farKey, v *view.View, banner bool, box plane.AABB, z Z,
	sprite render.SpriteID, wa *Atlas, id uid.UID64) {
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
				colour: colour})
			f.sums = append(f.sums, farSum{})
		}
		f.last, f.lastAt = key, g
	}
	f.frame.groups[g].count++
	mid := Position{AABB: box}.Center()
	sum := &f.sums[g]
	sum.x += float64(mid.X)
	sum.y += float64(mid.Y)
	sum.w += float64(box.Size.X)
	sum.h += float64(box.Size.Y)
	sum.altitude += float64(z.Altitude)
	sum.height += float64(z.Height)
	f.members = append(f.members, farMember{group: g, id: id})
}

// farSettle lays the groups the walk gathered out for the frame: each one's units in a run of
// ids in the order met, its centroid, mean size and mean Z
func (s *renderer) farSettle() {
	f := &s.far
	b := f.frame
	if b == nil || len(b.groups) == 0 {
		return
	}
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
	}
	if cap(b.ids) < int(from) {
		b.ids = make([]uid.UID64, from)
	}
	b.ids = b.ids[:from]
	for _, m := range f.members {
		b.ids[f.at[m.group]] = m.id
		f.at[m.group]++
	}
}

// box is the box g is drawn over
func (g *farGroup) box() plane.AABB {
	return plane.NewAABB(geom.NewVec(g.centre.X-g.size.X/2, g.centre.Y-g.size.Y/2), g.size.X,
		g.size.Y)
}

// banner is the Banner of g, its units b's
func (b *farFrame) banner(g *farGroup) Banner {
	return Banner{Kind: g.kind, Sector: int(g.sector), Center: g.centre,
		IDs: b.ids[g.from : g.from+g.count : g.from+g.count], Count: int(g.count),
		Box: g.box().AABB, Z: g.z}
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
	return level >= lod.Strategic && level < lod.Omitted && wa != nil && wa.declares(id, level)
}

// drawBanners draws the banners of the groups v drew, each a point of its kind's look at its
// level, fogged where it stands, noting the lowest of what is drawn as Compose does; it reports
// whether it drew a point
func (s *renderer) drawBanners(f *render.Frame, cam camera.Camera, figs *figures, v *view.View,
	levels byLevel, sorts bool, lowest *render.At, low *float32, any *bool) bool {
	drew := false
	for k := range figs.far.groups {
		g := &figs.far.groups[k]
		if g.view != v || !g.banner {
			continue
		}
		e := figure{box: g.box(), z: g.z}
		if !*any || sorts || g.z.Altitude < *low {
			at := f.Lifted(g.centre.X, g.centre.Y, g.z.Altitude, FigureLift)
			if !*any || at.Before(*lowest) {
				*lowest, *low, *any = at, g.z.Altitude, true
			}
		}
		look := [1]layer{{Appearance: render.Appearance{SpriteID: g.sprite}}}
		fog := v.Fog(g.centre.X, g.centre.Y)
		drew = s.point(f, cam, &e, e.box, look[:], levels, g.level, fog) || drew
	}
	return drew
}

// pixels: how far over the top of what it counts a count stands
const countAbove = 2

// most counts kept as text
const countsKept = 1 << 12

// DrawGlass draws on dst, the picture as shown through cam, where the counts are drawn, the number
// of units in each banner and in each kind's points two or more in a sector at the strategic
// scale, over the banner or by the points' centroid, in their kind's colour.
func (s *renderer) DrawGlass(dst *render.Image, cam camera.Camera) {
	figs := s.figures.Front()
	if !figs.far.counts || len(figs.far.groups) == 0 {
		return
	}
	v := figs.views[camera.OriginOf(cam)]
	vw, vh := cam.Viewport()
	if v == nil || vw <= 0 || vh <= 0 {
		return
	}
	b := dst.Bounds()
	// the camera's pixels may be fewer than those shown, the frame budget's
	kx, ky := float32(b.Dx())/vw, float32(b.Dy())/vh
	font := render.DefaultFont()
	for k := range figs.far.groups {
		g := &figs.far.groups[k]
		if g.view != v || !g.banner && g.count < 2 {
			continue
		}
		// over a banner's box, by the middle of the points
		top, over := g.z.Top(), g.centre.Y
		if g.banner {
			over -= g.size.Y / 2
		}
		if !camera.InFront(cam, g.centre.X, over, top) {
			continue
		}
		sx, sy := cam.Project(g.centre.X, over, top)
		label := s.countOf(g.count)
		w, h := font.Measure(label)
		x := float32(b.Min.X) + sx*kx - w/2
		y := float32(b.Min.Y) + sy*ky - h - countAbove
		if x+w < float32(b.Min.X) || x > float32(b.Max.X) || y+h < float32(b.Min.Y) ||
			y > float32(b.Max.Y) {
			continue
		}
		c := g.colour
		font.Draw(dst, label, x+1, y+1, color.RGBA{A: 255})
		font.Draw(dst, label, x, y, color.RGBA{R: c[0], G: c[1], B: c[2], A: 255})
	}
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
