package world

import (
	"math"
	"time"

	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/gram/plugins/world/view"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ render.Direct = (*renderer)(nil)

type renderer struct {
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[render.Appearance]
	z           goke.OptComp[Z]
	lies        goke.OptComp[Lie]
	active      goke.OptComp[cgoke.Active]
	lighting    Lighting
	rules       *render.Rules
	layers      [][]render.Appearance
	shown       []bool
	atlas       render.AtlasSource
	look        func() Look
	fog         *[3]float32 // the fog's colour where nothing else gives it
	shaded      map[render.SpriteID]render.MaterialID
	phased      map[render.SpriteID]phase // a material's effect, for how far it has run
	lying       map[render.SpriteID]bool
	standing    map[render.SpriteID]bool
	edges       map[render.SpriteID]render.SpriteID
	views       func(camera.Camera) *view.View
	view        *view.View
	// nil, the composer's own
	clock func() time.Duration

	bases   []Base
	actives []cgoke.Active
	quads   []camera.Quad
}

func newRenderer(atlas render.AtlasSource, views func(camera.Camera) *view.View, rules *render.Rules, look func() Look) *renderer {
	return &renderer{atlas: atlas, views: views, rules: rules, look: look}
}

func (s *renderer) shade(shaded map[render.SpriteID]render.MaterialID,
	phased map[render.SpriteID]phase) {
	s.shaded, s.phased = shaded, phased
}

func (s *renderer) lie(lying map[render.SpriteID]bool, edges map[render.SpriteID]render.SpriteID,
	standing map[render.SpriteID]bool) {
	s.lying, s.edges, s.standing = lying, edges, standing
}

func (s *renderer) Init(si *goke.SysInit) {
	// what lies beyond every band of the sectors is drawn by no camera
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z).Optional(&s.lies).
		Optional(&s.active).Exclude(goke.Exclude[sector.Off]())
	if s.lighting != nil {
		s.lighting.Join(qb)
	}
	render.Own(s.rules, &s.base)
	render.Own(s.rules, &s.appearance)
	render.Own(s.rules, &s.z)
	render.Own(s.rules, &s.active)
	s.rules.Bind(qb)
	s.renderQuery = qb.Build()
}

// Clock is the game time the frame's animations go by, the tactical clock's as shown.
func (s *renderer) Clock() (time.Duration, bool) {
	if s.clock == nil {
		return 0, false
	}
	return s.clock(), true
}

// Compose hands f every drawn entity in sight of cam as the world's Look lays it, lit as the
// Lighting says, in white light without one: the Look knows the wind, the world its entities.
func (s *renderer) Compose(f *render.Frame, cam camera.Camera) {
	if s.atlas == nil {
		return
	}
	if s.fog != nil && !f.Uniformed("Fog") {
		f.Uniform("Fog", s.fog[:]...)
	}
	look := s.look()
	if d, ok := look.(DirectLook); ok {
		d.Begin(f, cam)
	}
	lies, _ := look.(LyingLook)
	uprights, _ := look.(UprightLook)
	foggy, _ := look.(FoggyLook)
	if s.lighting != nil {
		s.lighting.Begin(f, cam)
	}
	s.view = s.views(cam)
	lowest, any := render.Nearest, false
	s.each(func(cur *goke.Cursor, i int, z *Z, lie *Lie) {
		box := s.bases[i].Pos.AABB
		dir := s.bases[i].Vel.Dir
		var stands Z
		if z != nil {
			stands = *z
		}
		mid := s.bases[i].Pos.Center()
		at := f.Lifted(float32(mid.X), float32(mid.Y), float32(stands.Altitude), FigureLift)
		if !any || at.Before(lowest) {
			lowest, any = at, true
		}
		// coarsely: no edges, flat where it would stand
		level := s.view.Level(mid.X, mid.Y)
		coarse := level == view.Coarse || level == view.Fogged
		if foggy != nil {
			foggy.Fog(s.view.Fog(mid.X, mid.Y))
		}
		lit := render.WhiteLight
		if s.lighting != nil {
			lit = s.lighting.Of(cur, i, box, stands, s.layers[i][0])
		}
		for _, l := range s.layers[i] {
			if m, ok := s.shaded[l.SpriteID]; ok {
				s.material(f, cam, &s.bases[i], at, m, s.progress(l.SpriteID, i))
				continue
			}
			if lies != nil && s.lying[l.SpriteID] && lie != nil && lie.Length > 0 {
				lain := *lie
				edge, ok := s.edges[l.SpriteID]
				lain.Edged = lain.Edged && ok && !coarse
				lies.Lay(f, cam, box, dir, lain, stands, s.atlas, l, edge, lit)
				continue
			}
			if lies != nil && s.lying[l.SpriteID] {
				lies.Lie(f, cam, box, stands, s.atlas, l, lit)
				continue
			}
			if uprights != nil && s.standing[l.SpriteID] && !coarse {
				uprights.Upright(f, cam, box, stands, s.atlas, l, lit)
				continue
			}
			look.Sprite(f, cam, box, stands, s.atlas, l, lit)
		}
	})
	if foggy != nil {
		foggy.Fog(0)
	}
	if _, ok := look.(DirectLook); ok && any {
		f.DrawAt(lowest) // the figures come after what lies beneath the lowest of them
	}
}

// Draw has a DirectLook draw the sprites Compose handed it.
func (s *renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if d, ok := s.look().(DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func (s *renderer) each(visit func(cur *goke.Cursor, i int, z *Z, lie *Lie)) {
	s.renderQuery.All()
	for s.renderQuery.Next() {
		cursor := s.renderQuery.Cursor()
		ids := cursor.IDs
		s.bases = s.base.Slice(cursor)
		s.actives = s.active.Slice(cursor)
		appearances := s.appearance.Slice(cursor)
		zs, lies := s.z.Slice(cursor), s.lies.Slice(cursor)

		for len(s.layers) < len(ids) {
			s.layers = append(s.layers, nil)
			s.shown = append(s.shown, false)
		}
		if !s.seenAny(ids) {
			continue
		}
		for i := range ids {
			s.layers[i] = append(s.layers[i][:0], appearances[i])
		}
		s.rules.Run(cursor, s.layers[:len(ids)], s.shown[:len(ids)])
		for i, id := range ids {
			if !s.shown[i] || !s.view.Contains(id) {
				continue
			}
			var z *Z
			if zs != nil {
				z = &zs[i]
			}
			var lie *Lie
			if lies != nil {
				lie = &lies[i]
			}
			visit(cursor, i, z, lie)
		}
	}
}

// whether the view holds any of ids, so the looks' rules run on a chunk worth drawing alone
func (s *renderer) seenAny(ids []uid.UID64) bool {
	if !s.view.Culled {
		return true
	}
	for _, id := range ids {
		if s.view.In.Has(id) {
			return true
		}
	}
	return false
}

// how far the effect a material is under has run on the entity i of the chunk, 0 for none
func (s *renderer) progress(id render.SpriteID, i int) float32 {
	ph, ok := s.phased[id]
	if !ok || s.actives == nil {
		return 0
	}
	left, on := s.actives[i].Left(ph.effect)
	return ph.progress(left, on)
}

// the material's inputs: Custom the box's middle, half its side and its heading in radians,
// Red how fast it moves in its own lengths a second up to 1, the fraction how far the effect it
// is under has run
func (s *renderer) material(
	f *render.Frame, cam camera.Camera, b *Base, at render.At, m render.MaterialID,
	progress float32,
) {
	box := b.Pos.AABB
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	angle := float32(0)
	if b.Vel.Dir.X != 0 || b.Vel.Dir.Y != 0 {
		angle = angleOf(b.Vel.Dir) * math.Pi / 180
	}
	speed := float32(0)
	if side := x1 - x0; side > 0 {
		speed = min(float32(b.Vel.Value)/side, 1)
	}
	custom := [4]float32{(x0 + x1) / 2, (y0 + y1) / 2, (x1 - x0) / 2, angle}
	o := render.Overlay{
		Material: m,
		World:    render.Box(x0, y0, x1, y1),
		Red:      [4]float32{speed, speed, speed, speed},
		Fraction: [4]float32{progress, progress, progress, progress},
		Custom:   [4][4]float32{custom, custom, custom, custom},
	}
	s.quads = cam.ToScreenQuads(x0, y0, x1, y1, s.quads[:0])
	for _, q := range s.quads {
		f.Material(at, render.Corners{{q.X0, q.Y0}, {q.X1, q.Y0}, {q.X0, q.Y1}, {q.X1, q.Y1}}, &o)
	}
}
