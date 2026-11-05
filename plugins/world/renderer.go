package world

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
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
	keen        goke.OptComp[sector.Keen]
	asleep      goke.OptComp[sector.L4]
	field       goke.OptComp[Field]
	unfolded    goke.OptComp[Unfolded]
	rules       *Rules
	layers      [][]render.Appearance
	shown       []bool
	actives     []cgoke.Active
	phased      map[render.SpriteID]phase // a material's effect, for how far it has run
	views       func() map[camera.Camera]*view.View
	seeing      []*view.View // the views published
	// the stretches unfolded, by whole and sector those Publish found, and whether it has yet
	stretches  *goke.Query
	stretch    goke.Comp[stretch]
	unfoldedIn []place
	found      bool
	// the sectors a box covers, nil in a world of none; a field's parts, and those recorded
	covers func(box geom.AABB, dst []sector.Part) []sector.Part
	parts  []sector.Part
	pieces []plane.AABB
	// by view published, what it keeps of each level's entities; thinned, whether any thins
	keeps   []keeping
	thinned bool
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
	// by sprite, whether it is a material: shaded's keys
	materials []bool
	// what stands at L2, a point a figure, and the light they are drawn in
	points render.Sprites
	litFor pointLight
	// the world's size and edges, for the sector an entity at a picture's edge is drawn by
	space SpaceCfg
	// the figures' colour summed into the ground, Read.Aggregate's; none without sectors
	aggregate aggregate
}

func newRenderer(atlas render.AtlasSource, views func() map[camera.Camera]*view.View,
	rules *Rules, look func() Look) *renderer {
	return &renderer{atlas: atlas, views: views, rules: rules, look: look}
}

func (s *renderer) shade(shaded map[render.SpriteID]render.MaterialID,
	phased map[render.SpriteID]phase) {
	s.shaded, s.phased = shaded, phased
	s.materials = s.materials[:0]
	for id := range shaded {
		for int(id) >= len(s.materials) {
			s.materials = append(s.materials, false)
		}
		s.materials[id] = true
	}
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
// Look knows the wind, the world its entities. What it sees at L2 it draws itself, a point each;
// a field over each sector of it, at that sector's level, casting nothing; what cam rides in,
// fastened Inside, it leaves out but for its shadow.
func (s *renderer) Compose(f *render.Frame, cam camera.Camera) {
	s.aggregate.upload()
	s.points.Reset()
	if s.atlas == nil {
		return
	}
	if s.fog != nil && !f.Uniformed("Fog") {
		f.Uniform("Fog", s.fog[:]...)
	}
	look := s.look()
	d, direct := look.(DirectLook)
	if direct {
		d.Begin(f, cam)
	}
	sp := s.space
	s.points.Wrap(float32(sp.Width), float32(sp.Height), sp.Edges.WrapsX(), sp.Edges.WrapsY())
	s.litFor.ok = false
	lies, _ := look.(LyingLook)
	uprights, _ := look.(UprightLook)
	foggy, _ := look.(FoggyLook)
	figs := s.figures.Front()
	v := figs.views[camera.OriginOf(cam)]
	var levels byLevel
	if wa, ok := s.atlas.(*Atlas); ok {
		levels = wa.levels
	}
	// a view sorting by height alone has the frame asked where a figure lies only when it is lower
	sorts := cam.Projection().Sorts()
	rider, inside := ridden(cam)
	lowest, low, any, pointed := render.Nearest, float32(0), false, false
	for k := range figs.all {
		e := &figs.all[k]
		if v == nil {
			continue
		}
		box, z, wide := e.box, e.z, e.piece
		var level lod.Level
		if !wide {
			if !v.Contains(e.id) {
				continue
			}
			level = s.level(v, box.AABB, Position{AABB: box}.Center())
		} else {
			var seen bool
			if level, seen = s.pieceAt(v, box); !seen {
				continue
			}
		}
		mid := Position{AABB: box}.Center()
		layers := figs.layers[e.from : e.from+int32(e.n)]
		if !levels.anyAt(layers, level) {
			continue
		}
		if id, ok := levels.at(layers[0].SpriteID, level); ok && level != lod.L2 && !wide {
			f.Cast(box.AABB, z.Altitude, z.Height, e.shadow, s.atlas, id)
		}
		if inside && e.id == rider {
			continue
		}
		if !any || sorts || z.Altitude < low {
			if at := f.Lifted(mid.X, mid.Y, z.Altitude, lifted(e)); !any || at.Before(lowest) {
				lowest, low, any = at, z.Altitude, true
			}
		}
		if level == lod.L2 {
			pointed = s.point(f, cam, e, box, layers, levels, v.Fog(mid.X, mid.Y)) || pointed
			continue
		}
		// coarsely: no edges, flat where it would stand
		coarse := level >= lod.L1
		if foggy != nil {
			foggy.Fog(v.Fog(mid.X, mid.Y))
		}
		lit := f.Lit(mid.X, mid.Y, z.Altitude, e.shadow.Take)
		for _, l := range layers {
			id, ok := levels.at(l.SpriteID, level)
			if !ok {
				continue
			}
			a := l.Appearance
			a.SpriteID = id
			if m, ok := s.shaded[id]; ok {
				s.material(f, cam, e, box, m, l.progress)
				continue
			}
			if lies != nil && s.lying[id] && e.lie > 0 && !wide {
				lain := figs.lies[e.lie-1]
				edge, ok := s.edges[id]
				lain.Edged = lain.Edged && ok && !coarse
				lies.Lay(f, cam, box, e.dir, lain, z, s.atlas, a, edge, lit)
				continue
			}
			// a field's piece lies on its ground, under what stands there
			if lies != nil && (s.lying[id] || wide) {
				lies.Lie(f, cam, box, z, s.atlas, a, lit)
				continue
			}
			if uprights != nil && s.standing[id] && !coarse && !wide {
				uprights.Upright(f, cam, box, z, s.atlas, a, lit)
				continue
			}
			look.Sprite(f, cam, box, z, s.atlas, a, lit)
		}
	}
	if foggy != nil {
		foggy.Fog(0)
	}
	if any && (direct || pointed) {
		f.DrawAt(lowest) // the figures come after what lies beneath the lowest of them
	}
}

// lifted is how far over its Altitude e lies among what is drawn back to front: a field's piece
// under the figures
func lifted(e *figure) float32 {
	if e.piece {
		return ilook.PieceLift
	}
	return FigureLift
}

// ridden is the entity cam is fastened Inside, whose figure an eye in it does not see
func ridden(cam camera.Camera) (uid.UID64, bool) {
	if c, ok := cam.(camera.Fastenable); ok {
		if at := c.Fastening(); at.How == camera.Inside {
			return at.Entity, true
		}
	}
	return 0, false
}

// point draws e at L2 over box as one point of the first of its layers drawn there, fogged by
// fog, a material among them as it is; no Look, no light of its own and no shadow. It reports
// whether it drew a point.
func (s *renderer) point(f *render.Frame, cam camera.Camera, e *figure, box plane.AABB,
	layers []layer, levels byLevel, fog float32) bool {
	pointed := false
	mid := Position{AABB: box}.Center()
	for _, l := range layers {
		id, ok := levels.at(l.SpriteID, lod.L2)
		switch {
		case !ok:
		case int(id) < len(s.materials) && s.materials[id]:
			s.material(f, cam, e, box, s.shaded[id], l.progress)
		case !pointed:
			pointed = true
			lit := s.pointLit(f, e.shadow.Take)
			s.points.Fog(fog)
			s.points.Take(lit.Takes)
			s.points.Direct(lit.Direct)
			z := e.z
			// seen at its middle, lying for the depth where a figure's sprite would; a field's
			// piece on its ground, under every figure
			seen, deep := z.Altitude+z.Height/2, z.Top()+FigureLift
			if e.piece {
				seen, deep = z.Altitude, z.Altitude+ilook.PieceLift
			}
			s.points.Point(s.atlas, id, mid.X, mid.Y, box.Size.X, box.Size.Y, seen, deep,
				lit.Light)
		}
	}
	return pointed
}

// pointLight is the light the points taking take are drawn in this frame, while ok
type pointLight struct {
	lit  render.Illumination
	take render.Taking
	ok   bool
}

// pointLit is the light a point taking take is drawn in: the distant light's alone, the lamps
// left to the finer levels, so the frame is asked once for a run of points taking alike
func (s *renderer) pointLit(f *render.Frame, take render.Taking) *render.Illumination {
	if !s.litFor.ok || s.litFor.take != take {
		s.relight(f, take)
	}
	return &s.litFor.lit
}

func (s *renderer) relight(f *render.Frame, take render.Taking) {
	s.litFor = pointLight{lit: f.Lit(0, 0, 0, lampless(take)), take: take, ok: true}
}

// lampless is take without the lamps' light, which hangs on where a thing stands; the lamps' alone
// becomes the clouds' shade, which no point takes, so the distant light still lights it
func lampless(take render.Taking) render.Taking {
	switch {
	case take&render.Shines != 0:
		return take
	case take == 0:
		take = render.TakesAll
	}
	if take &^= render.TakesLamps; take == 0 {
		return render.TakesClouds
	}
	return take
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
	if b.TopLeft.X <= mid.X && mid.X <= b.BottomRight.X && b.TopLeft.Y <= mid.Y &&
		mid.Y <= b.BottomRight.Y {
		if level := v.Level(mid.X, mid.Y); level != lod.L4 {
			return level
		}
	}
	w, h := float32(s.space.Width), float32(s.space.Height)
	wx, wy := s.space.Edges.WrapsX(), s.space.Edges.WrapsY()
	level := v.Level(held(mid.X, b.TopLeft.X, b.BottomRight.X, w, wx),
		held(mid.Y, b.TopLeft.Y, b.BottomRight.Y, h, wy))
	if level != lod.L4 {
		return level
	}
	if f := v.Fogging(); f[3] > 0 {
		dx, dy := apart(mid.X, f[0], w, wx), apart(mid.Y, f[1], h, wy)
		if f32.Hypot(dx, dy) > f[3] {
			return level
		}
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

// fieldBox is the box of size round own's middle
func fieldBox(own plane.AABB, size geom.Vec) plane.AABB {
	c := Position{AABB: own}.Center()
	return plane.NewAABB(geom.NewVec(c.X-size.X/2, c.Y-size.Y/2), size.X, size.Y)
}

// meets reports whether box meets v's bounds, round an axis of the world that wraps
func (s *renderer) meets(v *view.View, box geom.AABB) bool {
	if !v.Culled {
		return true
	}
	b, sp := v.Bounds, s.space
	return across(b.TopLeft.X, b.BottomRight.X, box.TopLeft.X, box.BottomRight.X,
		float32(sp.Width), sp.Edges.WrapsX()) &&
		across(b.TopLeft.Y, b.BottomRight.Y, box.TopLeft.Y, box.BottomRight.Y,
			float32(sp.Height), sp.Edges.WrapsY())
}

// across reports whether [a0, a1] and [b0, b1] overlap along an axis, round size where it wraps
func across(a0, a1, b0, b1, size float32, wraps bool) bool {
	switch {
	case !wraps || size <= 0:
		return a0 < b1 && b0 < a1
	case a1-a0 >= size || b1-b0 >= size:
		return true
	}
	// the copy of b beginning within a world past a0
	from := f32.Mod(b0-a0, size)
	if from < 0 {
		from += size
	}
	from += a0
	return from < a1 || from+(b1-b0)-size > a0
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

// Draw has a DirectLook draw the sprites Compose handed it, and draws the points after them.
func (s *renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if d, ok := s.look().(DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
	s.points.Draw(t, cam, u)
}

// the material's inputs: Custom the box's middle, half its side and its heading in radians,
// Red how fast it moves in its own lengths a second up to 1, the fraction how far the effect it
// is under has run
func (s *renderer) material(f *render.Frame, cam camera.Camera, e *figure, box plane.AABB,
	m render.MaterialID, progress float32) {
	x0, y0 := box.TopLeft.X, box.TopLeft.Y
	x1, y1 := box.BottomRight.X, box.BottomRight.Y
	mid := Position{AABB: box}.Center()
	at := f.Lifted(mid.X, mid.Y, e.z.Altitude, lifted(e))
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
