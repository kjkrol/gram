package world

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/view"
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
	// the world's size and edges, for the sector an entity at a picture's edge is drawn by
	space SpaceCfg
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

// Compose hands f every entity published in sight of cam in its kind's look at the level cam sees
// it at, as the world's Look lays it, lit as the Lighting says, in white light without one: the
// Look knows the wind, the world its entities.
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
	v := figs.views[camera.OriginOf(cam)]
	var levels byLevel
	if wa, ok := s.atlas.(*Atlas); ok {
		levels = wa.levels
	}
	lowest, any := render.Nearest, false
	for k := range figs.all {
		e := &figs.all[k]
		if v == nil || !v.Contains(e.id) {
			continue
		}
		box, z := e.box, e.z
		mid := Position{AABB: box}.Center()
		level := s.level(v, box.AABB, mid)
		layers := figs.layers[e.from : e.from+int32(e.n)]
		if !levels.anyAt(layers, level) {
			continue
		}
		at := f.Lifted(mid.X, mid.Y, z.Altitude, FigureLift)
		if !any || at.Before(lowest) {
			lowest, any = at, true
		}
		// coarsely: no edges, flat where it would stand
		coarse := level >= lod.L1
		if foggy != nil {
			foggy.Fog(v.Fog(mid.X, mid.Y))
		}
		lit := f.Lit(mid.X, mid.Y, z.Altitude, e.shadow.Take)
		if id, ok := levels.at(layers[0].SpriteID, level); ok {
			f.Cast(box.AABB, z.Altitude, z.Height, e.shadow, s.atlas, id)
		}
		for _, l := range layers {
			id, ok := levels.at(l.SpriteID, level)
			if !ok {
				continue
			}
			a := l.Appearance
			a.SpriteID = id
			if m, ok := s.shaded[id]; ok {
				s.material(f, cam, e, at, m, l.progress)
				continue
			}
			if lies != nil && s.lying[id] && e.lie > 0 {
				lain := figs.lies[e.lie-1]
				edge, ok := s.edges[id]
				lain.Edged = lain.Edged && ok && !coarse
				lies.Lay(f, cam, box, e.dir, lain, z, s.atlas, a, edge, lit)
				continue
			}
			if lies != nil && s.lying[id] {
				lies.Lie(f, cam, box, z, s.atlas, a, lit)
				continue
			}
			if uprights != nil && s.standing[id] && !coarse {
				uprights.Upright(f, cam, box, z, s.atlas, a, lit)
				continue
			}
			look.Sprite(f, cam, box, z, s.atlas, a, lit)
		}
	}
	if foggy != nil {
		foggy.Fog(0)
	}
	if _, ok := look.(DirectLook); ok && any {
		f.DrawAt(lowest) // the figures come after what lies beneath the lowest of them
	}
}

// level is how fully v draws what stands over box: as its middle's sector, the middle held within
// the view's bounds so what the view found at the picture's edge is drawn by a sector on it, or,
// that sector off a picture turned on the world and the middle within the sight, as the finest
// sector under the box's corners held so
func (s *renderer) level(v *view.View, box geom.AABB, mid geom.Vec) lod.Level {
	if v.Grid() == nil || !v.Culled {
		return v.Level(mid.X, mid.Y)
	}
	b := v.Bounds
	w, h := float32(s.space.Width), float32(s.space.Height)
	wx, wy := s.space.Edges.WrapsX(), s.space.Edges.WrapsY()
	level := v.Level(held(mid.X, b.TopLeft.X, b.BottomRight.X, w, wx),
		held(mid.Y, b.TopLeft.Y, b.BottomRight.Y, h, wy))
	if level != lod.L4 {
		return level
	}
	if f := v.Fogging(); f[3] > 0 && f32.Hypot(apart(mid.X, f[0], w, wx),
		apart(mid.Y, f[1], h, wy)) > f[3] {
		return level
	}
	for _, x := range [2]float32{box.TopLeft.X, box.BottomRight.X} {
		for _, y := range [2]float32{box.TopLeft.Y, box.BottomRight.Y} {
			level = min(level, v.Level(held(x, b.TopLeft.X, b.BottomRight.X, w, wx),
				held(y, b.TopLeft.Y, b.BottomRight.Y, h, wy)))
		}
	}
	return level
}

// held is the world coordinate x held within [lo, hi]: x where it lies there, on an axis wrapping
// round size where a copy of it does, else the end nearest to the nearest copy
func held(x, lo, hi, size float32, wraps bool) float32 {
	at := min(max(x, lo), hi)
	if !wraps || size <= 0 || at == x {
		return at
	}
	gap := f32.Abs(x - at)
	for _, c := range [2]float32{x - size, x + size} {
		if n := min(max(c, lo), hi); f32.Abs(c-n) < gap {
			gap, at = f32.Abs(c-n), n
		}
	}
	// the bounds may run past the world's edge, the grid's sectors not
	if at = f32.Mod(at, size); at < 0 {
		at += size
	}
	return at
}

// apart is how far a lies from b, the shorter way round an axis wrapping round size
func apart(a, b, size float32, wraps bool) float32 {
	d := f32.Abs(a - b)
	if wraps && size > 0 {
		d = f32.Mod(d, size)
		d = min(d, size-d)
	}
	return d
}

// anyAt says whether any of layers is drawn at level
func (b byLevel) anyAt(layers []layer, level lod.Level) bool {
	for _, l := range layers {
		if _, ok := b.at(l.SpriteID, level); ok {
			return true
		}
	}
	return false
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
	x0, y0 := e.box.TopLeft.X, e.box.TopLeft.Y
	x1, y1 := e.box.BottomRight.X, e.box.BottomRight.Y
	angle := float32(0)
	if e.dir != (geom.Vec{}) {
		angle = angleOf(e.dir) * math.Pi / 180
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
