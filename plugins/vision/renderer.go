package vision

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ConeStyle draws one entity's view, given the fan already projected to screen
// space: pts[0] is the observer, the rest its boundary in order.
type ConeStyle interface {
	Draw(screen *ebiten.Image, pts []ebiten.Vertex)
}

// ConeShader is a ConeStyle that also draws the shadows of a view — the ground out of sight — given
// as one path of closed quads on the screen; a style without it gets DefaultShadow.
type ConeShader interface {
	Shade(screen *ebiten.Image, shadows *vector.Path)
}

var shadowColor = color.RGBA{R: 10, G: 10, B: 20, A: 110}

// DefaultShadow fills the shadows of a view with a dark veil.
func DefaultShadow(screen *ebiten.Image, shadows *vector.Path) {
	vector.FillPath(screen, shadows, &vector.FillOptions{}, &vector.DrawPathOptions{ColorScale: colorScaleOf(shadowColor), AntiAlias: true})
}

// ConeStyleFn adapts a plain function to ConeStyle.
type ConeStyleFn func(screen *ebiten.Image, pts []ebiten.Vertex)

func (f ConeStyleFn) Draw(screen *ebiten.Image, pts []ebiten.Vertex) { f(screen, pts) }

var _ ConeStyle = ConeStyleFn(nil)

var coneColor = color.RGBA{R: 255, G: 220, B: 90, A: 160}

// DefaultConeStyle strokes the boundary of the lit region and leaves the inside clear.
func DefaultConeStyle() ConeStyle {
	return ConeStyleFn(func(screen *ebiten.Image, pts []ebiten.Vertex) {
		var path vector.Path
		path.MoveTo(pts[0].DstX, pts[0].DstY)
		for _, p := range pts[1:] {
			path.LineTo(p.DstX, p.DstY)
		}
		path.Close()
		vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: 1}, &vector.DrawPathOptions{
			ColorScale: colorScaleOf(coneColor),
			AntiAlias:  true,
		})
	})
}

func colorScaleOf(c color.RGBA) ebiten.ColorScale {
	var cs ebiten.ColorScale
	cs.ScaleWithColor(c)
	return cs
}

var _ render.Renderer = (*Renderer)(nil)

// Renderer draws the view of every entity carrying SightOutline.
type Renderer struct {
	camera camera.Camera
	space  *aabbworld.Space
	style  ConeStyle

	worldW, worldH float32
	wraps          bool

	// groundOf finds the world's Ground when drawing starts; a fan is draped over it.
	groundOf func() world.Ground
	ground   world.Ground
	grounded bool

	query *goke.Query
	base  goke.Comp[world.Base]
	sight goke.Comp[Sight]
	out   goke.Comp[SightOutline]
	z     goke.OptComp[world.Z]

	pts    []ebiten.Vertex // rebuilt per entity, kept to stay off the heap
	shades vector.Path
}

// NewRenderer builds a Renderer with DefaultConeStyle, wrapping cones at the edges of space.
func NewRenderer(cam camera.Camera, space *aabbworld.Space) *Renderer {
	w, h, edges := space.Bounds()
	return &Renderer{
		camera: cam, space: space, style: DefaultConeStyle(),
		worldW: float32(w), worldH: float32(h), wraps: edges&aabbworld.Torus != 0,
	}
}

// WithGround has the fans follow the ground heights groundOf gives when drawing starts.
func (r *Renderer) WithGround(groundOf func() world.Ground) *Renderer {
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

func (r *Renderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.sight, &r.out).Optional(&r.z).Build()
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	if !r.grounded {
		r.grounded = true
		if r.groundOf != nil {
			r.ground = r.groundOf()
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
			r.drawCone(screen, &bases[i].Pos, alt, &sights[i], &outlines[i])
		}
	}
}

// drawCone draws one entity's view once per image of the world it reaches into; on a world that
// does not wrap the fan is draped over the ground from the observer's altitude.
func (r *Renderer) drawCone(screen *ebiten.Image, pos *world.Position, alt float32, s *Sight, o *SightOutline) {
	ox, oy := centreOf(pos)

	if !r.wraps {
		r.style.Draw(screen, r.draped(float32(ox), float32(oy), alt, s, o))
		r.shade(screen, float32(ox), float32(oy), s, o)
		return
	}
	sx, sy := r.camera.ToScreen(float32(ox), float32(oy))

	zoom := r.camera.Zoom()
	box := r.space.WrapAABB(r.coneBox(ox, oy, s.Radius))
	render.VisitWrapImages(box, r.worldW, r.worldH, func(_ geom.AABB, dx, dy float32) bool {
		r.style.Draw(screen, r.fan(sx+dx*zoom, sy+dy*zoom, s, o))
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

// draped projects the fan point by point: the apex at the observer's altitude, the boundary on the
// ground under it (flat at 0 without a Ground).
func (r *Renderer) draped(ox, oy, alt float32, s *Sight, o *SightOutline) []ebiten.Vertex {
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * s.HalfAngle / float64(o.Count-1)

	ax, ay := r.camera.Project(ox, oy, alt)
	r.pts = append(r.pts[:0], ebiten.Vertex{DstX: ax, DstY: ay})
	for i := range int(o.Count) {
		a := facing - s.HalfAngle + float64(i)*step
		d := float64(o.Depths[i])
		x, y := ox+float32(d*math.Cos(a)), oy+float32(d*math.Sin(a))
		z := float32(0)
		if r.ground != nil {
			z = float32(r.ground.At(geom.NewVec(float64(x), float64(y))))
		}
		px, py := r.camera.Project(x, y, z)
		r.pts = append(r.pts, ebiten.Vertex{DstX: px, DstY: py})
	}
	return r.pts
}

// shade draws the shadows of the view: for every angle, each band as a quad spanning halfway to the
// angles either side, draped over the ground.
func (r *Renderer) shade(screen *ebiten.Image, ox, oy float32, s *Sight, o *SightOutline) {
	n := int(o.Count)
	if n < 2 {
		return
	}
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * s.HalfAngle / float64(n-1)
	r.shades.Reset()
	drawn := false
	for i := range n {
		for _, b := range o.Shadows[i] {
			if b == (Band{}) {
				continue
			}
			a := facing - s.HalfAngle + float64(i)*step
			lo, hi := max(a-step/2, facing-s.HalfAngle), min(a+step/2, facing+s.HalfAngle)
			p0x, p0y := r.onGround(ox, oy, lo, b.From)
			p1x, p1y := r.onGround(ox, oy, hi, b.From)
			p2x, p2y := r.onGround(ox, oy, hi, b.To)
			p3x, p3y := r.onGround(ox, oy, lo, b.To)
			r.shades.MoveTo(p0x, p0y)
			r.shades.LineTo(p1x, p1y)
			r.shades.LineTo(p2x, p2y)
			r.shades.LineTo(p3x, p3y)
			r.shades.Close()
			drawn = true
		}
	}
	if !drawn {
		return
	}
	if sh, ok := r.style.(ConeShader); ok {
		sh.Shade(screen, &r.shades)
		return
	}
	DefaultShadow(screen, &r.shades)
}

// onGround is the screen point dist away from (ox, oy) at angle a, on the ground there.
func (r *Renderer) onGround(ox, oy float32, a float64, dist float32) (float32, float32) {
	x, y := ox+dist*float32(math.Cos(a)), oy+dist*float32(math.Sin(a))
	z := float32(0)
	if r.ground != nil {
		z = float32(r.ground.At(geom.NewVec(float64(x), float64(y))))
	}
	return r.camera.Project(x, y, z)
}

// fan rebuilds the boundary around an already-projected anchor from the stored reaches, for the
// images of a cone on a wrapping world.
func (r *Renderer) fan(sx, sy float32, s *Sight, o *SightOutline) []ebiten.Vertex {
	facing := math.Atan2(s.Facing.Y, s.Facing.X)
	step := 2 * s.HalfAngle / float64(o.Count-1)
	zoom := r.camera.Zoom()

	r.pts = append(r.pts[:0], ebiten.Vertex{DstX: sx, DstY: sy})
	for i := range int(o.Count) {
		a := facing - s.HalfAngle + float64(i)*step
		d := float64(o.Depths[i])
		r.pts = append(r.pts, ebiten.Vertex{
			DstX: sx + float32(d*math.Cos(a))*zoom,
			DstY: sy + float32(d*math.Sin(a))*zoom,
		})
	}
	return r.pts
}

func centreOf(p *world.Position) (float64, float64) {
	box := p.AABB.AABB
	return (float64(box.TopLeft.X) + float64(box.BottomRight.X)) / 2,
		(float64(box.TopLeft.Y) + float64(box.BottomRight.Y)) / 2
}
