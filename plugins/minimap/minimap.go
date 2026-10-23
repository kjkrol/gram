package minimap

import (
	"image/color"
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

var outlineColor = color.RGBA{R: 240, G: 240, B: 240, A: 230}

// the marks under the outline of the camera's view
const (
	marksLevel   = 1
	outlineLevel = 2
	outlineWidth = 1.5
)

// minimap is the minimap of one camera
type minimap struct {
	p       *Plugin
	of      camera.Camera
	issuer  command.Issuer
	lens    *lens
	element *ui.Element
	marks   *marksView
}

// follow puts the lens over the camera's middle, turned as it looks where turning
func (m *minimap) follow(turning bool) {
	x, y := m.middle()
	ax, ay := float32(0), float32(-1)
	if h, ok := m.of.(camera.Headed); ok && turning {
		ax, ay = h.Ahead()
	}
	m.lens.aim(x, y, ax, ay)
}

func (m *minimap) middle() (float32, float32) {
	if f, ok := m.of.(camera.Focused); ok {
		x, y, _ := f.Focus()
		return x, y
	}
	b := m.of.Bounds()
	return float32(b.TopLeft.X+b.BottomRight.X) / 2, float32(b.TopLeft.Y+b.BottomRight.Y) / 2
}

// redraw is how often at most a minimap's marks are published, and the minimap drawn anew: in
// between it shows its last picture
const redraw = time.Second / 30

// paced is a minimap's feed drawn anew as its marks are published
type paced struct {
	*render.Feed
	marks   *marksView
	w, h    int
	last    *render.Image
	version uint64
}

// Resize is laid out every frame: a picture of another size is drawn anew at once
func (p *paced) Resize(w, h int) {
	if w != p.w || h != p.h {
		p.w, p.h, p.last = w, h, nil
	}
	p.Feed.Resize(w, h)
}

func (p *paced) Draw() *render.Image {
	if v := p.marks.drawn.Front().version; p.last == nil || v != p.version {
		p.last, p.version = p.Feed.Draw(), v
	}
	return p.last
}

// the minimap's own picture: the ground, the marks and the outline of the camera's view
func (m *minimap) picture() *render.Composer {
	m.marks = &marksView{m: m}
	return render.NewComposer(&groundView{m: m}, m.marks, &outlineView{m: m})
}

// allMarks are the marks of every minimap, made with the Stage's scenes: readied at its Setup,
// published once a frame
type allMarks struct{ p *Plugin }

func (s allMarks) SetupSystems() []goke.System {
	var out []goke.System
	for _, m := range s.p.maps {
		out = append(out, goke.SystemFn{OnInit: m.marks.Init})
	}
	return out
}

// Publish takes every minimap's marks once a redraw at most, while the minimaps are shown.
func (s allMarks) Publish() {
	if s.p.Res.Settings.Hidden {
		return
	}
	now := time.Now()
	for _, m := range s.p.maps {
		m.marks.publish(now)
	}
}

// Swap has the marks published last drawn.
func (s allMarks) Swap() {
	for _, m := range s.p.maps {
		m.marks.swap()
	}
}

// groundView lays the ground's miniature under the minimap, a copy a world away round a torus
type groundView struct{ m *minimap }

var _ render.Direct = (*groundView)(nil)

func (g *groundView) Compose(f *render.Frame, _ camera.Camera) {
	if g.m.p.ground != nil {
		f.DrawOpaque()
	}
}

func (g *groundView) Draw(t render.Target, _ camera.Camera, _ render.Uniforms) {
	img, perUnit := g.m.p.ground.Miniature()
	l := g.m.lens
	if img == nil || perUnit <= 0 || t.Screen == nil || l.scale == 0 {
		return
	}
	turn := math.Atan2(float64(-l.ax), float64(-l.ay))
	for kx := -1; kx <= 1; kx++ {
		for ky := -1; ky <= 1; ky++ {
			if kx != 0 && !l.wrapX || ky != 0 && !l.wrapY {
				continue
			}
			var op render.DrawImageOptions
			op.Filter = render.FilterLinear
			op.GeoM.Scale(1/float64(perUnit), 1/float64(perUnit))
			op.GeoM.Translate(float64(kx)*l.world.X-float64(l.cx),
				float64(ky)*l.world.Y-float64(l.cy))
			op.GeoM.Rotate(turn)
			op.GeoM.Scale(float64(l.scale), float64(l.scale))
			op.GeoM.Translate(float64(l.w)/2, float64(l.h)/2)
			t.Screen.DrawImage(img, &op)
		}
	}
}

// marksView draws a dot for every entity round the minimap whose kind has a Mark, as published
type marksView struct {
	m     *minimap
	q     *goke.Query
	base  goke.Comp[world.Base]
	app   goke.Comp[render.Appearance]
	found view.EntitySet
	ids   []uid.UID64
	marks []knownMark // by sprite
	drawn render.Buffered[dots]
	due   time.Time
	fresh bool // published since the last swap
}

// dots are the marks round a minimap where they stand, as of the version-th publish
type dots struct {
	all     []dot
	version uint64
}

type dot struct {
	x, y float32
	mark Mark
}

func (v *marksView) Init(si *goke.SysInit) {
	v.q = si.NewQueryBuilder(&v.base, &v.app).Build()
}

// publish takes the marks round the minimap once their time comes
func (v *marksView) publish(now time.Time) {
	if now.Before(v.due) {
		return
	}
	v.due, v.fresh = now.Add(redraw), true
	b := v.drawn.Back()
	b.all, b.version = b.all[:0], v.drawn.Front().version+1
	styles := v.m.p.w.Read(0).Styles()
	if styles == nil {
		return
	}
	v.found.Clear()
	v.ids = v.ids[:0]
	// a box across a seam is met once a piece
	v.m.p.w.Read(0).Space().Query(v.m.lens.covered(), aabbworld.AnyCapability, func(id uid.UID64) {
		if !v.found.Has(id) {
			v.found.Add(id)
			v.ids = append(v.ids, id)
		}
	})
	for _, id := range v.ids {
		if !v.q.Seek(id) {
			continue
		}
		cur := v.q.Cursor()
		mark, ok := v.markOf(styles, v.app.At(cur).SpriteID)
		if !ok || mark.Size <= 0 {
			continue
		}
		c := v.base.At(cur).Pos.Center()
		b.all = append(b.all, dot{x: float32(c.X), y: float32(c.Y), mark: mark})
	}
}

func (v *marksView) swap() {
	if v.fresh {
		v.drawn.Swap()
		v.fresh = false
	}
}

func (v *marksView) Compose(f *render.Frame, _ camera.Camera) {
	for _, d := range v.drawn.Front().all {
		mark := d.mark
		sx, sy := v.m.lens.Project(d.x, d.y, 0)
		r := mark.Size / 2
		if sx+r < 0 || sx-r > v.m.lens.w || sy+r < 0 || sy-r > v.m.lens.h {
			continue
		}
		dot := render.Corners{
			{sx - r, sy - r},
			{sx + r, sy - r},
			{sx - r, sy + r},
			{sx + r, sy + r},
		}
		f.Quad(render.OnTop(marksLevel), dot, [4]color.RGBA{mark.Color, mark.Color, mark.Color,
			mark.Color})
	}
}

// the mark of the sprite id, looked up once: the styles stay as the atlas closed them
func (v *marksView) markOf(styles *render.Styles, id render.SpriteID) (Mark, bool) {
	for int(id) >= len(v.marks) {
		v.marks = append(v.marks, knownMark{})
	}
	k := &v.marks[id]
	if !k.known {
		k.mark, k.ok = render.StyleOf[Mark](styles, id)
		k.known = true
	}
	return k.mark, k.ok
}

type knownMark struct {
	mark      Mark
	ok, known bool
}

// outlineView draws what the minimap's camera shows of the ground
type outlineView struct {
	m       *minimap
	corners [4][2]float32
}

func (o *outlineView) Compose(f *render.Frame, _ camera.Camera) {
	w, h := o.m.of.Viewport()
	if w == 0 || h == 0 {
		return
	}
	for k, s := range [4][2]float32{{0, 0}, {w, 0}, {w, h}, {0, h}} {
		x, y := o.m.ground(s[0], s[1])
		o.corners[k][0], o.corners[k][1] = o.m.lens.Project(x, y, 0)
	}
	for k := range o.corners {
		a, b := o.corners[k], o.corners[(k+1)%len(o.corners)]
		f.Line(render.OnTop(outlineLevel), a[0], a[1], b[0], b[1], outlineWidth, outlineColor)
	}
}

// ground is the world point the camera's screen point (sx, sy) sees on the ground; one past the
// horizon the way it runs, twice the minimap's reach from the middle
func (m *minimap) ground(sx, sy float32) (float32, float32) {
	p, ok := m.of.(camera.Picker)
	if !ok {
		return m.of.Unproject(sx, sy, 0)
	}
	x, y, _, hit := p.Pick(sx, sy)
	if hit {
		return x, y
	}
	cx, cy := m.middle()
	dx, dy := float64(x-cx), float64(y-cy)
	n := math.Hypot(dx, dy)
	if far := 2 * m.lens.reach; n > far {
		dx, dy = dx*far/n, dy*far/n
	}
	return cx + float32(dx), cy + float32(dy)
}

// clicks takes a press on the minimap: its camera goes to the world point under it
type clicks struct{ m *minimap }

var _ ui.Clicked = (*clicks)(nil)

func (c *clicks) Over(geom.AABB, render.Surface) {}

func (c *clicks) Click(at geom.Vec) {
	l := c.m.lens
	x, y := l.FromScreen(float32(at.X), float32(at.Y))
	c.m.p.w.Write().Carrier().Put(c.m.issuer, cameras.CenterAt{
		Camera: c.m.of,
		X:      wrapped(float64(x), l.world.X, l.wrapX),
		Y:      wrapped(float64(y), l.world.Y, l.wrapY),
	})
}

// v round into [0, size) where the axis wraps
func wrapped(v, size float64, wraps bool) float64 {
	if !wraps {
		return v
	}
	return v - size*math.Floor(v/size)
}
