package selection

import (
	"image/color"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// HighlightStyle composes one Selected entity's outline, given its world-space AABB and the altitude
// it stands at (0 in a flat world); it belongs on the Marks tier, over everything.
type HighlightStyle interface {
	Compose(f *render.Frame, cam camera.Camera, box camera.AABB, altitude float32)
}

// HighlightStyleFn adapts a plain function to HighlightStyle.
type HighlightStyleFn func(f *render.Frame, cam camera.Camera, box camera.AABB, altitude float32)

func (fn HighlightStyleFn) Compose(f *render.Frame, cam camera.Camera, box camera.AABB, altitude float32) {
	fn(f, cam, box, altitude)
}

var _ HighlightStyle = HighlightStyleFn(nil)

var highlightColor = color.RGBA{R: 220, G: 40, B: 40, A: 255}

// DefaultHighlightStyle draws a thin red outline around box; through an isometric camera the box
// is the diamond on the ground under the entity.
func DefaultHighlightStyle() HighlightStyle {
	var quads []camera.Quad
	return HighlightStyleFn(func(f *render.Frame, cam camera.Camera, box camera.AABB, altitude float32) {
		x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
		x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
		if _, iso := cam.Projection().(camera.Isometric); iso {
			c := render.ProjectCorners(cam, x0, y0, x1, y1, altitude)
			outline(f, [4][2]float32{c[0], c[1], c[3], c[2]}, 2, highlightColor)
			return
		}
		quads = cam.ToScreenQuads(x0, y0, x1, y1, quads[:0])
		for _, q := range quads {
			outline(f, [4][2]float32{{q.X0, q.Y0}, {q.X1, q.Y0}, {q.X1, q.Y1}, {q.X0, q.Y1}}, 2, highlightColor)
		}
	})
}

// outline draws the closed polygon round pts on the Marks tier.
func outline(f *render.Frame, pts [4][2]float32, width float32, c color.RGBA) {
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		f.Line(render.Marks, 0, p[0], p[1], q[0], q[1], width, c)
	}
}

// Renderer outlines every Selected entity and, when the plugin built it, the box being dragged in
// the viewport's camera.
type Renderer struct {
	style    HighlightStyle
	marquees *marquees

	query    *goke.Query
	base     goke.Comp[world.Base]
	marks    goke.Comp[plugin.Tags[Family]]
	z        goke.OptComp[world.Z]
	selected plugin.Tag[Family]
}

var _ render.Source = (*Renderer)(nil)

// NewRenderer builds a Renderer with DefaultHighlightStyle.
func NewRenderer(selected plugin.Tag[Family]) *Renderer {
	return &Renderer{style: DefaultHighlightStyle(), selected: selected}
}

// WithStyle overrides how the highlight is drawn — the escape hatch for a custom HighlightStyle.
func (r *Renderer) WithStyle(style HighlightStyle) *Renderer {
	r.style = style
	return r
}

func (r *Renderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.marks).Optional(&r.z).Build()
}

// Compose outlines the Selected entities through cam, and the box being dragged in it, on the Marks
// tier.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	r.query.All()
	for r.query.Next() {
		cursor := r.query.Cursor()
		bases := r.base.Slice(cursor)
		marks := r.marks.Slice(cursor)
		zs := r.z.Slice(cursor)
		for i := range cursor.IDs {
			if marks[i].Has(r.selected) {
				alt := float32(0)
				if zs != nil {
					alt = float32(zs[i].Altitude)
				}
				r.style.Compose(f, cam, bases[i].Pos.AABB.AABB, alt)
			}
		}
	}
	if r.marquees != nil {
		r.marquees.compose(f, cam)
	}
}
