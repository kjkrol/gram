package vision

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ConePoint is a point of a view's outline on screen, with the depth of the world point under it
// and how many screen units a world unit spans there (camera.ScaleAt; 0 not in front of the eye).
type ConePoint struct{ X, Y, Depth, Scale float32 }

// ConeStyle composes one entity's view from its outline: a closed ring starting at the observer,
// out along one edge of the cone, round its boundary and back along the other edge, draped over
// the ground; the edges come in steps of the ground, so each piece of the ring has a depth of its
// own.
type ConeStyle interface {
	Compose(f *render.Frame, outline []ConePoint)
}

// ConeStyleFn adapts a plain function to ConeStyle.
type ConeStyleFn func(f *render.Frame, outline []ConePoint)

func (fn ConeStyleFn) Compose(f *render.Frame, outline []ConePoint) { fn(f, outline) }

var _ ConeStyle = ConeStyleFn(nil)

var coneColor = color.RGBA{R: 255, G: 220, B: 90, A: 160}

// DefaultConeStyle strokes the ring on the Overlays tier and leaves the inside clear; each stroke
// takes the depth of its nearer end, so it is drawn over the ground it lies on and what stands in
// front of that ground hides it.
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

var _ render.Source = (*Renderer)(nil)

// Renderer is the render.Source of the view of every entity carrying SightOutline, on the Overlays
// tier: its outline in a ConeStyle and, in a world with heights, the ground out of sight in a Shadow.
type Renderer struct {
	camera camera.Camera // the one of the frame being composed
	frame  *render.Frame
	space  *aabbworld.Space
	style  ConeStyle
	shadow Shadow

	worldW, worldH float32
	wraps          bool

	// groundOf finds the world's Ground when composing starts; a view is draped over it in pieces
	// of step.
	groundOf func() board.Heights
	ground   board.Heights
	step     float32
	grounded bool

	query *goke.Query
	base  goke.Comp[world.Base]
	sight goke.Comp[Sight]
	out   goke.Comp[SightOutline]
	z     goke.OptComp[world.Z]

	pts []ConePoint // rebuilt per entity, kept to stay off the heap
}

// NewRenderer builds a Renderer with DefaultConeStyle and DefaultShadow, wrapping cones at the
// edges of space.
func NewRenderer(space *aabbworld.Space) *Renderer {
	w, h, edges := space.Bounds()
	return &Renderer{
		space: space, style: DefaultConeStyle(), shadow: DefaultShadow,
		worldW: float32(w), worldH: float32(h), wraps: edges&aabbworld.Torus != 0,
	}
}

// WithGround has the views follow the ground heights groundOf gives when composing starts.
func (r *Renderer) WithGround(groundOf func() board.Heights) *Renderer {
	r.groundOf = groundOf
	return r
}

// Style reports how cones are currently drawn.
func (r *Renderer) Style() ConeStyle { return r.style }

// WithStyle replaces how each cone is drawn.
func (r *Renderer) WithStyle(style ConeStyle) *Renderer {
	r.style = style
	return r
}

// WithShadow replaces how the ground out of sight is shaded.
func (r *Renderer) WithShadow(shadow Shadow) *Renderer {
	r.shadow = shadow
	return r
}

func (r *Renderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.sight, &r.out).Optional(&r.z).Build()
}

// Compose hands f every view in sight of cam.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	r.frame, r.camera = f, cam
	if !r.grounded {
		r.grounded = true
		if r.groundOf != nil {
			r.ground = r.groundOf()
		}
		if r.ground != nil {
			r.step = float32(r.ground.Step())
		}
	}
	r.query.All()
	for r.query.Next() {
		cursor := r.query.Cursor()
		bases := r.base.Slice(cursor)
		sights := r.sight.Slice(cursor)
		outlines := r.out.Slice(cursor)
		zs := r.z.Slice(cursor)

		for i := range cursor.IDs {
			if outlines[i].Count < 2 || !r.camera.Visible(bases[i].Pos.AABB.AABB) {
				continue
			}
			alt := float32(0)
			if zs != nil {
				alt = float32(zs[i].Altitude)
			}
			r.cone(&bases[i].Pos, alt, &sights[i], &outlines[i])
		}
	}
}

// cone composes one entity's view once per image of the world it reaches into; on a world that
// does not wrap it is draped over the ground from the observer's altitude, shadows included.
func (r *Renderer) cone(pos *world.Position, alt float32, s *Sight, o *SightOutline) {
	ox, oy := centreOf(pos)

	if !r.wraps {
		r.style.Compose(r.frame, r.draped(float32(ox), float32(oy), alt, s, o))
		r.shade(float32(ox), float32(oy), s, o)
		return
	}
	sx, sy := r.camera.ToScreen(float32(ox), float32(oy))

	zoom := r.camera.Zoom()
	box := r.space.WrapAABB(r.coneBox(ox, oy, s.Radius))
	render.VisitWrapImages(box, r.worldW, r.worldH, func(_ geom.AABB, dx, dy float32) bool {
		r.style.Compose(r.frame, r.fan(sx+dx*zoom, sy+dy*zoom, s, o))
		return true
	})
}

// coneBox is the square a cone covers, clamped to the size of the world.
func (r *Renderer) coneBox(ox, oy, radius float64) geom.AABB {
	w, h := float64(r.worldW), float64(r.worldH)
	return geom.NewAABBAt(
		geom.NewVec(ox-radius, oy-radius),
		math.Min(2*radius, w), math.Min(2*radius, h),
	)
}

// draped is the view's ring point by point: the apex at the observer's altitude, the rest on the
// ground (flat at 0 without a Ground), each edge of the cone in steps of the ground.
func (r *Renderer) draped(ox, oy, alt float32, s *Sight, o *SightOutline) []ConePoint {
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * s.HalfAngle / float64(o.Count-1)
	n := int(o.Count)

	ax, ay := r.camera.Project(ox, oy, alt)
	r.pts = append(r.pts[:0], ConePoint{X: ax, Y: ay, Depth: r.camera.Depth(ox, oy, alt)})
	first, last := facing-s.HalfAngle, facing-s.HalfAngle+float64(n-1)*step
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

// shade composes the shadows of the view: for every angle, each band as a strip spanning halfway
// to the angles either side, cut in steps of the ground and draped over it, fading in at its near
// and far ends and at a side where the next angle has no shadow there.
func (r *Renderer) shade(ox, oy float32, s *Sight, o *SightOutline) {
	n := int(o.Count)
	if n < 2 {
		return
	}
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * s.HalfAngle / float64(n-1)
	for i := range n {
		for _, b := range o.Shadows[i] {
			if b == (Band{}) {
				continue
			}
			a := facing - s.HalfAngle + float64(i)*step
			lo, hi := max(a-step/2, facing-s.HalfAngle), min(a+step/2, facing+s.HalfAngle)
			piece := b.To - b.From
			if r.step > 0 {
				piece = r.step
			}
			for d0 := b.From; d0 < b.To; d0 += piece {
				d1 := min(d0+piece, b.To)
				p0, p1 := r.onGround(ox, oy, lo, d0), r.onGround(ox, oy, hi, d0)
				p2, p3 := r.onGround(ox, oy, lo, d1), r.onGround(ox, oy, hi, d1)
				if p0.Scale == 0 || p1.Scale == 0 || p2.Scale == 0 || p3.Scale == 0 {
					continue // under the eye, or behind it: nothing of it is seen
				}
				// soft over the pixels the fade spans where the piece is drawn
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

// shadowed reports whether angle i of o holds a shadow anywhere along the stretch d0 to d1.
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

// onGround is the point dist away from (ox, oy) at angle a, on the ground there: on screen, with
// its depth.
func (r *Renderer) onGround(ox, oy float32, a float64, dist float32) ConePoint {
	x, y := ox+dist*float32(math.Cos(a)), oy+dist*float32(math.Sin(a))
	z := float32(0)
	if r.ground != nil {
		z = float32(r.ground.At(geom.NewVec(float64(x), float64(y))))
	}
	sx, sy := r.camera.Project(x, y, z)
	return ConePoint{X: sx, Y: sy, Depth: r.camera.Depth(x, y, z), Scale: camera.ScaleAt(r.camera, x, y, z)}
}

// fan rebuilds the ring round an already-projected anchor from the stored reaches, for the images
// of a cone on a wrapping world, seen from above.
func (r *Renderer) fan(sx, sy float32, s *Sight, o *SightOutline) []ConePoint {
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * s.HalfAngle / float64(o.Count-1)
	zoom := r.camera.Zoom()

	r.pts = append(r.pts[:0], ConePoint{X: sx, Y: sy})
	for i := range int(o.Count) {
		a := facing - s.HalfAngle + float64(i)*step
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
