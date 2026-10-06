package world

import (
	"math"
	"time"

	"github.com/kjkrol/gram/plugins/world/view"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*renderer)(nil)

// renderer is the render.Source of the Position+Appearance entities in the View of the viewport's
// camera — what it sees this tick — each laid on the screen by the world's Look, running the
// render.Rules — the effects' swaps and what the world's Atlas declared — over each chunk to
// settle their layers and which are drawn. A
// Stage that has not ticked yet sees everything. It is a render.Direct at render.Objects too, where a DirectLook draws the
// sprites it was handed.
type renderer struct {
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[render.Appearance]
	z           goke.OptComp[Z]
	rules       *render.Rules
	layers      [][]render.Appearance // one per entity of the chunk being drawn
	shown       []bool                // the chunk's, as the rules say
	atlas       render.AtlasSource
	look        func() Look
	shaded      map[render.SpriteID]render.MaterialID // the sprites worked out per pixel (the Atlas's)
	views       func(camera.Camera) *view.View
	view        *view.View // the one being drawn
	// clock is the game time the frame's animations go by; nil, the composer's own
	clock func() time.Duration

	bases []Base
	quads []camera.Quad // reused for the material quads
}

func newRenderer(atlas render.AtlasSource, views func(camera.Camera) *view.View, rules *render.Rules, look func() Look) *renderer {
	return &renderer{atlas: atlas, views: views, rules: rules, look: look}
}

// shade is the per-pixel looks the world's Atlas declared, by sprite; nil for none.
func (s *renderer) shade(shaded map[render.SpriteID]render.MaterialID) { s.shaded = shaded }

func (s *renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z)
	render.Own(s.rules, &s.base)
	render.Own(s.rules, &s.appearance)
	render.Own(s.rules, &s.z)
	s.rules.Bind(qb)
	s.renderQuery = qb.Build()
}

// Clock is the game time the frame's animations go by: the world's tactical clock's as shown
// (clock.Clock.Shown), so what sways and flows moves every frame, stands in the tactical pause and
// hurries with the tempo.
func (s *renderer) Clock() (time.Duration, bool) {
	if s.clock == nil {
		return 0, false
	}
	return s.clock(), true
}

// Compose hands f every drawn entity in sight of cam, as the world's Look lays it, in white light
// and swaying as its Appearance says: the Look — a view plugin's, the atmosphere's — knows the
// light and the wind; the world knows its entities.
func (s *renderer) Compose(f *render.Frame, cam camera.Camera) {
	look := s.look()
	if d, ok := look.(DirectLook); ok {
		d.Begin(cam)
	}
	s.view = s.views(cam)
	s.each(func(i int, z *Z) {
		box := s.bases[i].Pos.AABB
		if !cam.Visible(box.AABB) {
			return
		}
		var stands Z
		if z != nil {
			stands = *z
		}
		for _, l := range s.layers[i] {
			if m, ok := s.shaded[l.SpriteID]; ok {
				s.material(f, cam, &s.bases[i], m)
				continue
			}
			look.Sprite(f, cam, box, stands, s.atlas, l, render.Light{1, 1, 1})
		}
	})
}

// Tier is where a DirectLook's sprites come in the picture: render.Objects.
func (s *renderer) Tier() render.Tier { return render.Objects }

// Draw has a DirectLook draw the sprites Compose handed it; any other Look laid them on the frame.
func (s *renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if d, ok := s.look().(DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

// each walks the drawn entities of the View, their rules run, calling visit once per entity shown
// with its index in the chunk and its Z, nil without one.
func (s *renderer) each(visit func(i int, z *Z)) {
	s.renderQuery.All()
	for s.renderQuery.Next() {
		cursor := s.renderQuery.Cursor()
		ids := cursor.IDs
		s.bases = s.base.Slice(cursor)
		appearances := s.appearance.Slice(cursor)
		zs := s.z.Slice(cursor)

		for len(s.layers) < len(ids) {
			s.layers = append(s.layers, nil)
			s.shown = append(s.shown, false)
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
			visit(i, z)
		}
	}
}

// material lays the entity's per-pixel look: one quad of its box for m to work out, the
// material's inputs the entity's own state — the box in Custom (its middle, half its side and
// the way it is headed, radians in the engine's convention), and in Red how fast it moves, in
// its own lengths a second up to 1: 0 stands and breathes, 1 runs flat out. One piece per wrap image; drawn where the sprites are, at render.Objects.
func (s *renderer) material(f *render.Frame, cam camera.Camera, b *Base, m render.MaterialID) {
	box := b.Pos.AABB
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	angle := float32(0)
	if b.Vel.Dir.X != 0 || b.Vel.Dir.Y != 0 {
		angle = angleOf(b.Vel.Dir) * math.Pi / 180
	}
	speed := float32(0)
	if side := x1 - x0; side > 0 { // how fast, in its own lengths a second, up to 1
		speed = min(float32(b.Vel.Value)/side, 1)
	}
	custom := [4]float32{(x0 + x1) / 2, (y0 + y1) / 2, (x1 - x0) / 2, angle}
	o := render.Overlay{
		Material: m,
		World:    render.Box(x0, y0, x1, y1),
		Red:      [4]float32{speed, speed, speed, speed},
		Custom:   [4][4]float32{custom, custom, custom, custom},
	}
	s.quads = cam.ToScreenQuads(x0, y0, x1, y1, s.quads[:0])
	for _, q := range s.quads {
		f.Material(render.Objects, 0, render.Corners{{q.X0, q.Y0}, {q.X1, q.Y0}, {q.X0, q.Y1}, {q.X1, q.Y1}}, &o)
	}
}
