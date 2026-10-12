package world

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world/view"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*renderer)(nil)

type renderer struct {
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[render.Appearance]
	z           goke.OptComp[Z]
	lies        goke.OptComp[Lie]
	rules       *render.Rules
	layers      [][]render.Appearance
	shown       []bool
	atlas       render.AtlasSource
	look        func() Look
	shaded      map[render.SpriteID]render.MaterialID
	lying       map[render.SpriteID]bool
	standing    map[render.SpriteID]bool
	edges       map[render.SpriteID]render.SpriteID
	views       func(camera.Camera) *view.View
	view        *view.View
	// nil, the composer's own
	clock func() time.Duration

	bases []Base
	quads []camera.Quad
}

func newRenderer(atlas render.AtlasSource, views func(camera.Camera) *view.View, rules *render.Rules, look func() Look) *renderer {
	return &renderer{atlas: atlas, views: views, rules: rules, look: look}
}

func (s *renderer) shade(shaded map[render.SpriteID]render.MaterialID) { s.shaded = shaded }

func (s *renderer) lie(lying map[render.SpriteID]bool, edges map[render.SpriteID]render.SpriteID,
	standing map[render.SpriteID]bool) {
	s.lying, s.edges, s.standing = lying, edges, standing
}

func (s *renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z).Optional(&s.lies)
	render.Own(s.rules, &s.base)
	render.Own(s.rules, &s.appearance)
	render.Own(s.rules, &s.z)
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

// Compose hands f every drawn entity in sight of cam as the world's Look lays it, in white light:
// the Look knows the light and the wind, the world its entities.
func (s *renderer) Compose(f *render.Frame, cam camera.Camera) {
	look := s.look()
	if d, ok := look.(DirectLook); ok {
		d.Begin(cam)
	}
	lies, _ := look.(LyingLook)
	uprights, _ := look.(UprightLook)
	s.view = s.views(cam)
	s.each(func(i int, z *Z, lie *Lie) {
		box := s.bases[i].Pos.AABB
		dir := s.bases[i].Vel.Dir
		if !cam.Visible(spread(box.AABB, dir, lie)) {
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
			if lies != nil && s.lying[l.SpriteID] && lie != nil && lie.Length > 0 {
				lain := *lie
				edge, ok := s.edges[l.SpriteID]
				lain.Edged = lain.Edged && ok
				lies.Lay(f, cam, box, dir, lain, stands, s.atlas, l, edge, render.Light{1, 1, 1})
				continue
			}
			if lies != nil && s.lying[l.SpriteID] {
				lies.Lie(f, cam, box, stands, s.atlas, l, render.Light{1, 1, 1})
				continue
			}
			if uprights != nil && s.standing[l.SpriteID] {
				uprights.Upright(f, cam, box, stands, s.atlas, l, render.Light{1, 1, 1})
				continue
			}
			look.Sprite(f, cam, box, stands, s.atlas, l, render.Light{1, 1, 1})
		}
	})
}

// Tier is where a DirectLook's sprites come in the picture.
func (s *renderer) Tier() render.Tier { return render.Objects }

// Draw has a DirectLook draw the sprites Compose handed it.
func (s *renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if d, ok := s.look().(DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func spread(box geom.AABB, dir geom.Vec, lie *Lie) geom.AABB {
	if lie == nil || lie.Length <= 0 {
		return box
	}
	m := geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2)
	for _, c := range lie.Corners(m, dir) {
		box.TopLeft = geom.NewVec(min(box.TopLeft.X, c.X), min(box.TopLeft.Y, c.Y))
		box.BottomRight = geom.NewVec(max(box.BottomRight.X, c.X), max(box.BottomRight.Y, c.Y))
	}
	return box
}

func (s *renderer) each(visit func(i int, z *Z, lie *Lie)) {
	s.renderQuery.All()
	for s.renderQuery.Next() {
		cursor := s.renderQuery.Cursor()
		ids := cursor.IDs
		s.bases = s.base.Slice(cursor)
		appearances := s.appearance.Slice(cursor)
		zs, lies := s.z.Slice(cursor), s.lies.Slice(cursor)

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
			var lie *Lie
			if lies != nil {
				lie = &lies[i]
			}
			visit(i, z, lie)
		}
	}
}

// the material's inputs: Custom the box's middle, half its side and its heading in radians,
// Red how fast it moves in its own lengths a second up to 1
func (s *renderer) material(f *render.Frame, cam camera.Camera, b *Base, m render.MaterialID) {
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
		Custom:   [4][4]float32{custom, custom, custom, custom},
	}
	s.quads = cam.ToScreenQuads(x0, y0, x1, y1, s.quads[:0])
	for _, q := range s.quads {
		f.Material(render.Objects, 0, render.Corners{{q.X0, q.Y0}, {q.X1, q.Y0}, {q.X0, q.Y1}, {q.X1, q.Y1}}, &o)
	}
}
