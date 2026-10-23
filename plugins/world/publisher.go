package world

import (
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
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

// figure is an entity drawn, in float32: its box's corners, size and how far it runs past the
// world's far edges, its heading and speed, its Z, its layers figures.layers[from:from+n], its
// Lie figures.lies[lie-1] (0 for none), its mark the Lighting's
type figure struct {
	id                   uid.UID64
	x0, y0, x1, y1, w, h float32
	ox, oy               float32
	dx, dy, speed        float32
	alt, height, step    float32
	from, lie            int32
	n, mark              uint16
}

// the box of e as the Look takes it
func (e *figure) box() plane.AABB {
	return plane.AABB{
		AABB: geom.AABB{TopLeft: geom.Vec{X: float64(e.x0), Y: float64(e.y0)},
			BottomRight: geom.Vec{X: float64(e.x1), Y: float64(e.y1)}},
		Size:     geom.Vec{X: float64(e.w), Y: float64(e.h)},
		Overhang: geom.Vec{X: float64(e.ox), Y: float64(e.oy)},
	}
}

func (e *figure) stands() Z {
	return Z{Altitude: float64(e.alt), Height: float64(e.height), Step: float64(e.step)}
}

// layer is a sprite an entity is drawn with and how far the effect its material is under has run
type layer struct {
	render.Appearance
	progress float32
}

func (s *renderer) Init(si *goke.SysInit) {
	// what lies beyond every band of the sectors is drawn by no camera
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z).Optional(&s.lies).
		Optional(&s.active).Exclude(goke.Exclude[sector.Off]())
	if s.lighting != nil {
		s.lighting.Join(qb)
	}
	Own(s.rules, &s.base)
	Own(s.rules, &s.appearance)
	Own(s.rules, &s.z)
	Own(s.rules, &s.active)
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
	for cam, v := range s.views() {
		if b.views == nil {
			b.views = map[camera.Camera]*view.View{}
		}
		c := b.views[cam]
		if c == nil {
			c = &view.View{}
			b.views[cam] = c
		}
		v.CopyInto(c)
		s.seeing = append(s.seeing, c)
	}
	if s.atlas == nil || s.renderQuery == nil || len(s.seeing) == 0 {
		return
	}
	for s.renderQuery.All(); s.renderQuery.Next(); {
		s.publish(b, s.renderQuery.Cursor())
	}
}

// Swap has what was published last drawn.
func (s *renderer) Swap() { s.figures.Swap() }

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
	for i, id := range ids {
		if ruled && !s.shown[i] || !s.sees(id) {
			continue
		}
		n := len(b.all)
		b.all = slices.Grow(b.all, 1)[:n+1]
		e := &b.all[n]
		box, vel := &bases[i].Pos.AABB, &bases[i].Vel
		e.id = id
		e.x0, e.y0 = float32(box.TopLeft.X), float32(box.TopLeft.Y)
		e.x1, e.y1 = float32(box.BottomRight.X), float32(box.BottomRight.Y)
		e.w, e.h, e.ox, e.oy = float32(box.Size.X), float32(box.Size.Y),
			float32(box.Overhang.X), float32(box.Overhang.Y)
		e.dx, e.dy, e.speed = float32(vel.Dir.X), float32(vel.Dir.Y), float32(vel.Value)
		e.alt, e.height, e.step, e.lie, e.mark = 0, 0, 0, 0, 0
		if zs != nil {
			e.alt, e.height, e.step = float32(zs[i].Altitude), float32(zs[i].Height),
				float32(zs[i].Step)
		}
		if lies != nil && lies[i].Length > 0 {
			b.lies = append(b.lies, lies[i])
			e.lie = int32(len(b.lies))
		}
		if s.lighting != nil {
			e.mark = s.lighting.Mark(cur, i)
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
