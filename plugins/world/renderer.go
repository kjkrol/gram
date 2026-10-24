package world

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/gram/plugins/world/view"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*renderer)(nil)
var _ render.Overlying = (*renderer)(nil)

type renderer struct {
	// what Publish reads (publisher.go)
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[render.Appearance]
	z           goke.OptComp[Z]
	lies        goke.OptComp[Lie]
	active      goke.OptComp[cgoke.Active]
	rules       *Rules
	layers      [][]render.Appearance
	shown       []bool
	actives     []cgoke.Active
	phased      map[render.SpriteID]phase // a material's effect, for how far it has run
	views       func() map[camera.Camera]*view.View
	seeing      []*view.View // the views published
	// nil, the composer's own
	clock func() time.Duration

	figures render.Buffered[figures]

	lighting Lighting
	atlas    render.AtlasSource
	look     func() Look
	fog      *[3]float32 // the fog's colour where nothing else gives it
	shaded   map[render.SpriteID]render.MaterialID
	lying    map[render.SpriteID]bool
	standing map[render.SpriteID]bool
	edges    map[render.SpriteID]render.SpriteID
	quads    []camera.Quad
}

func newRenderer(atlas render.AtlasSource, views func() map[camera.Camera]*view.View,
	rules *Rules, look func() Look) *renderer {
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

// Clock is the game time the frame's animations go by, the tactical clock's as shown.
func (s *renderer) Clock() (time.Duration, bool) {
	f := s.figures.Front()
	return f.shown, f.clocked
}

func (*renderer) Overlies() {}

// Compose hands f every entity published in sight of cam as the world's Look lays it, lit as
// the Lighting says, in white light without one: the Look knows the wind, the world its entities.
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
	figs := s.figures.Front()
	v := figs.views[cam]
	lowest, any := render.Nearest, false
	for k := range figs.all {
		e := &figs.all[k]
		if v == nil || !v.Contains(e.id) {
			continue
		}
		box, z := e.box(), e.stands()
		mid := Position{AABB: box}.Center()
		at := f.Lifted(float32(mid.X), float32(mid.Y), e.alt, FigureLift)
		if !any || at.Before(lowest) {
			lowest, any = at, true
		}
		// coarsely: no edges, flat where it would stand
		level := v.Level(mid.X, mid.Y)
		coarse := level == view.Coarse || level == view.Fogged
		if foggy != nil {
			foggy.Fog(v.Fog(mid.X, mid.Y))
		}
		layers := figs.layers[e.from : e.from+int32(e.n)]
		lit := f.Lit(float32(box.TopLeft.X+box.Size.X/2), float32(box.TopLeft.Y+box.Size.Y/2),
			e.alt, e.shadow.Take)
		f.Cast(box.AABB, z.Altitude, z.Height, e.shadow, s.atlas, layers[0].SpriteID)
		for _, l := range layers {
			if m, ok := s.shaded[l.SpriteID]; ok {
				s.material(f, cam, e, at, m, l.progress)
				continue
			}
			if lies != nil && s.lying[l.SpriteID] && e.lie > 0 {
				lain := figs.lies[e.lie-1]
				edge, ok := s.edges[l.SpriteID]
				lain.Edged = lain.Edged && ok && !coarse
				dir := geom.Vec{X: float64(e.dx), Y: float64(e.dy)}
				lies.Lay(f, cam, box, dir, lain, z, s.atlas, l.Appearance, edge, lit)
				continue
			}
			if lies != nil && s.lying[l.SpriteID] {
				lies.Lie(f, cam, box, z, s.atlas, l.Appearance, lit)
				continue
			}
			if uprights != nil && s.standing[l.SpriteID] && !coarse {
				uprights.Upright(f, cam, box, z, s.atlas, l.Appearance, lit)
				continue
			}
			look.Sprite(f, cam, box, z, s.atlas, l.Appearance, lit)
		}
	}
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

// the material's inputs: Custom the box's middle, half its side and its heading in radians,
// Red how fast it moves in its own lengths a second up to 1, the fraction how far the effect it
// is under has run
func (s *renderer) material(
	f *render.Frame, cam camera.Camera, e *figure, at render.At, m render.MaterialID,
	progress float32,
) {
	x0, y0, x1, y1 := e.x0, e.y0, e.x1, e.y1
	angle := float32(0)
	if e.dx != 0 || e.dy != 0 {
		angle = angleOf(geom.Vec{X: float64(e.dx), Y: float64(e.dy)}) * math.Pi / 180
	}
	speed := float32(0)
	if side := x1 - x0; side > 0 {
		speed = min(e.speed/side, 1)
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
