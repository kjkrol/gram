package vision

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ConePoint is a point of a view's outline on screen, with the depth of the world point under it
// and how many screen units a world unit spans there (camera.ScaleAt; 0 not in front of the eye).
type ConePoint struct{ X, Y, Depth, Scale float32 }

// ConeStyle composes one entity's view from its outline: a closed ring starting at the observer,
// out along one edge of the cone, round its boundary and back along the other edge. Each point
// has the depth of the ground under it.
type ConeStyle interface {
	Compose(f *render.Frame, outline []ConePoint)
}

// ConeStyleFn adapts a plain function to ConeStyle.
type ConeStyleFn func(f *render.Frame, outline []ConePoint)

func (fn ConeStyleFn) Compose(f *render.Frame, outline []ConePoint) { fn(f, outline) }

var _ ConeStyle = ConeStyleFn(nil)

var coneColor = color.RGBA{R: 255, G: 220, B: 90, A: 160}

// DefaultConeStyle strokes the ring on the Overlays tier and leaves the inside clear.
func DefaultConeStyle() ConeStyle {
	return ConeStyleFn(func(f *render.Frame, outline []ConePoint) {
		for i, p := range outline {
			q := outline[(i+1)%len(outline)]
			f.Line(render.Overlays, max(p.Depth, q.Depth), p.X, p.Y, q.X, q.Y, 1, coneColor)
		}
	})
}

// Shadow is how the ground out of sight is shaded: a veil of Color over it, fading in over Fade
// world units wherever it meets ground in sight.
type Shadow struct {
	Color color.RGBA
	Fade  float32
}

// DefaultShadow is a dark veil fading in over a few units.
var DefaultShadow = Shadow{Color: color.RGBA{R: 10, G: 10, B: 20, A: 110}, Fade: 6}

var _ render.Direct = (*Renderer)(nil)

// Renderer draws the views: through a camera with Rays and in the default style every observer's
// on the GPU on ViewTier, the ground out of sight veiled in a Shadow; otherwise, as a
// render.Source on the Overlays tier, the outline of every entity carrying SightOutline in a
// ConeStyle.
type Renderer struct {
	camera camera.Camera
	frame  *render.Frame
	space  *aabbworld.Space
	style  ConeStyle
	shadow Shadow

	worldW, worldH float32
	wraps          bool
	hidden         bool

	groundOf func() ground.Heights
	ground   ground.Heights
	step     float32
	grounded bool

	query      *goke.Query
	base       goke.Comp[world.Base]
	sight      goke.Comp[Sight]
	eye        goke.Comp[world.Eye]
	out        goke.OptComp[SightOutline]
	z          goke.OptComp[world.Z]
	drawing    *render.Rules
	shown      []bool
	groundStep float32 // world units; 0 the ground's own

	pts []ConePoint // kept to stay off the heap

	gpu     *views
	onGPU   bool
	coverOf func() ground.Cover
	cover   ground.Cover
	bend    float64
	custom  bool
	wrap    [2]bool
}

// NewRenderer builds a Renderer with DefaultConeStyle and DefaultShadow over space.
func NewRenderer(space *aabbworld.Space) *Renderer {
	w, h, edges := space.Bounds()
	return &Renderer{
		space: space, style: DefaultConeStyle(), shadow: DefaultShadow,
		worldW: float32(w), worldH: float32(h), wraps: edges&aabbworld.Torus != 0, wrap: [2]bool{edges.WrapsX(), edges.WrapsY()}, gpu: newViews(),
	}
}

// WithCover has the views drawn on the GPU dimmed by the cover coverOf gives.
func (r *Renderer) WithCover(coverOf func() ground.Cover) *Renderer {
	r.coverOf = coverOf
	return r
}

// WithScale sinks the ground under an eye's level as far off as it lies, as scale's sight does.
func (r *Renderer) WithScale(scale world.Scale) *Renderer {
	r.bend = scale.Bend()
	return r
}

// WithGround has the views follow the ground groundOf gives.
func (r *Renderer) WithGround(groundOf func() ground.Heights) *Renderer {
	r.groundOf = groundOf
	return r
}

// Style is how cones are drawn.
func (r *Renderer) Style() ConeStyle { return r.style }

// Hide has the Renderer compose nothing until shown again.
func (r *Renderer) Hide(hidden bool) { r.hidden = hidden }

// Hidden reports whether the views are hidden.
func (r *Renderer) Hidden() bool { return r.hidden }

// WithStyle replaces how each cone is drawn; the views are then composed from each entity's
// SightOutline in every world.
func (r *Renderer) WithStyle(style ConeStyle) *Renderer {
	r.style, r.custom = style, true
	return r
}

// WithShadow replaces how the ground out of sight is shaded.
func (r *Renderer) WithShadow(shadow Shadow) *Renderer {
	r.shadow = shadow
	return r
}

// WithGroundStep drapes the views over the ground every step world units, in place of the
// ground's own step.
func (r *Renderer) WithGroundStep(step float64) *Renderer {
	r.groundStep = float32(step)
	return r
}

// WithDrawing has only the views of the observers a render.Show rule picks drawn; with none,
// every one is.
func (r *Renderer) WithDrawing(rules *render.Rules) *Renderer {
	r.drawing = rules
	return r
}

func (r *Renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&r.base, &r.sight, &r.eye).Optional(&r.out).Optional(&r.z)
	if r.drawing != nil {
		render.Own(r.drawing, &r.base)
		render.Own(r.drawing, &r.sight)
		render.Own(r.drawing, &r.eye)
		render.Own(r.drawing, &r.out)
		render.Own(r.drawing, &r.z)
		r.drawing.Bind(qb)
	}
	r.query = qb.Build()
}

// Tier is ViewTier.
func (r *Renderer) Tier() render.Tier { return ViewTier }

// world units, where a world without heights reads its cover
const levelStep = 8

// ViewTier is where the views drawn on the GPU come: over the ground, under what stands on it.
const ViewTier = render.Ground + 50

// Draw draws the views Compose noted on the GPU; nothing where it composed them.
func (r *Renderer) Draw(t render.Target, cam camera.Camera, _ render.Uniforms) {
	if r.hidden || !r.onGPU {
		return
	}
	r.gpu.draw(t, cam, r.ground, r.cover, r.worldW, r.worldH, r.step, r.wrap, r.bend, r.shadow)
}

func (r *Renderer) settle(cursor *goke.Cursor) {
	r.shown = r.shown[:0]
	for range cursor.IDs {
		r.shown = append(r.shown, true)
	}
	if r.drawing != nil {
		r.drawing.Run(cursor, nil, r.shown)
	}
}

// Compose hands f every view in sight of cam, or notes them for Draw; nothing while hidden.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	r.onGPU = false
	if r.hidden {
		return
	}
	r.frame, r.camera = f, cam
	if !r.grounded {
		r.grounded = true
		if r.groundOf != nil {
			r.ground = r.groundOf()
		}
		if r.coverOf != nil {
			r.cover = r.coverOf()
		}
		r.step = levelStep
		if r.ground != nil {
			r.step = float32(r.ground.Step())
		}
		if r.groundStep > 0 {
			r.step = r.groundStep
		}
	}
	_, rays := cam.(camera.Rays)
	r.onGPU = rays && !r.custom
	r.query.All()
	for r.query.Next() {
		cursor := r.query.Cursor()
		bases := r.base.Slice(cursor)
		sights := r.sight.Slice(cursor)
		eyes := r.eye.Slice(cursor)
		outlines := r.out.Slice(cursor)
		zs := r.z.Slice(cursor)
		r.settle(cursor)
		for i := range cursor.IDs {
			if !r.shown[i] {
				continue
			}
			var z world.Z
			if zs != nil {
				z = zs[i]
			}
			// over level ground only the outline holds the entities cutting the view
			outlined := outlines != nil && outlines[i].Count >= 2
			if r.onGPU && (r.ground != nil || !outlined) {
				ox, oy := centreOf(&bases[i].Pos)
				f := sights[i].Looking(bases[i].Vel.Dir)
				r.gpu.look(cam, observer{X: float32(ox), Y: float32(oy), Eye: float32(eyes[i].Level(z)), Reach: float32(sights[i].Radius),
					Facing: float32(math.Atan2(f.Y, f.X)), Half: float32(eyes[i].Angle / 2)})
				continue
			}
			if !outlined || !r.camera.Visible(bases[i].Pos.AABB.AABB) {
				continue
			}
			look := sights[i]
			look.Facing = look.Looking(bases[i].Vel.Dir)
			r.cone(&bases[i].Pos, float32(z.Altitude), eyes[i].Angle/2, &look, &outlines[i])
		}
	}
}

func (r *Renderer) cone(pos *world.Position, alt float32, half float64, s *Sight, o *SightOutline) {
	ox, oy := centreOf(pos)

	if !r.wraps {
		r.style.Compose(r.frame, r.draped(float32(ox), float32(oy), alt, half, s, o))
		r.shade(float32(ox), float32(oy), half, s, o)
		return
	}
	sx, sy := r.camera.ToScreen(float32(ox), float32(oy))

	zoom := r.camera.Zoom()
	box := r.space.WrapAABB(r.coneBox(ox, oy, s.Radius))
	render.VisitWrapImages(box, r.worldW, r.worldH, func(_ geom.AABB, dx, dy float32) bool {
		r.style.Compose(r.frame, r.fan(sx+dx*zoom, sy+dy*zoom, half, s, o))
		return true
	})
}

func (r *Renderer) coneBox(ox, oy, radius float64) geom.AABB {
	w, h := float64(r.worldW), float64(r.worldH)
	return geom.NewAABBAt(
		geom.NewVec(ox-radius, oy-radius),
		math.Min(2*radius, w), math.Min(2*radius, h),
	)
}

func (r *Renderer) draped(ox, oy, alt float32, half float64, s *Sight, o *SightOutline) []ConePoint {
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * half / float64(o.Count-1)
	n := int(o.Count)

	ax, ay := r.camera.Project(ox, oy, alt)
	r.pts = append(r.pts[:0], ConePoint{X: ax, Y: ay, Depth: r.camera.Depth(ox, oy, alt)})
	first, last := facing-half, facing-half+float64(n-1)*step
	for d := r.step; r.step > 0 && d < o.Depths[0]; d += r.step {
		r.pts = append(r.pts, r.onGround(ox, oy, first, d))
	}
	for i := range n {
		r.pts = append(r.pts, r.onGround(ox, oy, first+float64(i)*step, o.Depths[i]))
	}
	back := len(r.pts)
	for d := r.step; r.step > 0 && d < o.Depths[n-1]; d += r.step {
		r.pts = append(r.pts, r.onGround(ox, oy, last, d))
	}
	// the way back down the far edge runs towards the observer
	for i, j := back, len(r.pts)-1; i < j; i, j = i+1, j-1 {
		r.pts[i], r.pts[j] = r.pts[j], r.pts[i]
	}
	return r.pts
}

func (r *Renderer) shade(ox, oy float32, half float64, s *Sight, o *SightOutline) {
	n := int(o.Count)
	if n < 2 {
		return
	}
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * half / float64(n-1)
	for i := range n {
		for _, b := range o.Shadows[i] {
			if b == (Band{}) {
				continue
			}
			a := facing - half + float64(i)*step
			lo, hi := max(a-step/2, facing-half), min(a+step/2, facing+half)
			piece := b.To - b.From
			if r.step > 0 {
				piece = r.step
			}
			for d0 := b.From; d0 < b.To; d0 += piece {
				d1 := min(d0+piece, b.To)
				p0, p1 := r.onGround(ox, oy, lo, d0), r.onGround(ox, oy, hi, d0)
				p2, p3 := r.onGround(ox, oy, lo, d1), r.onGround(ox, oy, hi, d1)
				if p0.Scale == 0 || p1.Scale == 0 || p2.Scale == 0 || p3.Scale == 0 {
					continue // under the eye or behind it
				}
				fade := r.shadow.Fade * (p0.Scale + p1.Scale + p2.Scale + p3.Scale) / 4
				var edges render.Fade
				if d0 == b.From {
					edges.Top = fade
				}
				if d1 == b.To {
					edges.Bottom = fade
				}
				if !shadowed(o, i-1, d0, d1) {
					edges.Left = fade
				}
				if !shadowed(o, i+1, d0, d1) {
					edges.Right = fade
				}
				depth := max(p0.Depth, p1.Depth, p2.Depth, p3.Depth) // over every tile it lies on
				r.frame.Soft(render.Overlays, depth, render.Corners{{p0.X, p0.Y}, {p1.X, p1.Y}, {p2.X, p2.Y}, {p3.X, p3.Y}}, r.shadow.Color, edges)
			}
		}
	}
}

func shadowed(o *SightOutline, i int, d0, d1 float32) bool {
	if i < 0 || i >= int(o.Count) {
		return false
	}
	for _, b := range o.Shadows[i] {
		if b != (Band{}) && b.From < d1 && b.To > d0 {
			return true
		}
	}
	return false
}

func (r *Renderer) onGround(ox, oy float32, a float64, dist float32) ConePoint {
	x, y := ox+dist*float32(math.Cos(a)), oy+dist*float32(math.Sin(a))
	z := float32(0)
	if r.ground != nil {
		z = float32(r.ground.At(geom.NewVec(float64(x), float64(y))))
	}
	sx, sy := r.camera.Project(x, y, z)
	return ConePoint{X: sx, Y: sy, Depth: r.camera.Depth(x, y, z), Scale: camera.ScaleAt(r.camera, x, y, z)}
}

func (r *Renderer) fan(sx, sy float32, half float64, s *Sight, o *SightOutline) []ConePoint {
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * half / float64(o.Count-1)
	zoom := r.camera.Zoom()

	r.pts = append(r.pts[:0], ConePoint{X: sx, Y: sy})
	for i := range int(o.Count) {
		a := facing - half + float64(i)*step
		d := float64(o.Depths[i])
		r.pts = append(r.pts, ConePoint{
			X: sx + float32(d*math.Cos(a))*zoom,
			Y: sy + float32(d*math.Sin(a))*zoom,
		})
	}
	return r.pts
}

func centreOf(p *world.Position) (float64, float64) {
	box := p.AABB.AABB
	return (float64(box.TopLeft.X) + float64(box.BottomRight.X)) / 2,
		(float64(box.TopLeft.Y) + float64(box.BottomRight.Y)) / 2
}
