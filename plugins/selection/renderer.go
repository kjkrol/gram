package selection

import (
	"image/color"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/render"
)

// HighlightStyle composes one Selected entity's outline from its footprint — the ground under it
// on screen, as the world's Look lays it, in pieces where it crosses a wrap seam; it belongs on the
// Marks tier, over everything.
type HighlightStyle interface {
	Compose(f *render.Frame, footprint []render.Corners)
}

// HighlightStyleFn adapts a plain function to HighlightStyle.
type HighlightStyleFn func(f *render.Frame, footprint []render.Corners)

func (fn HighlightStyleFn) Compose(f *render.Frame, footprint []render.Corners) { fn(f, footprint) }

var _ HighlightStyle = HighlightStyleFn(nil)

var highlightColor = color.RGBA{R: 220, G: 40, B: 40, A: 255}

// DefaultHighlightStyle draws a thin red outline round every piece of the footprint.
func DefaultHighlightStyle() HighlightStyle {
	return HighlightStyleFn(func(f *render.Frame, footprint []render.Corners) {
		for _, c := range footprint {
			outline(f, [4][2]float32{c[0], c[1], c[3], c[2]}, 2, highlightColor)
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
	style     HighlightStyle
	marquees  *marquees
	look      func() world.Look
	footprint []render.Corners

	query    *goke.Query
	base     goke.Comp[world.Base]
	marks    goke.Comp[tag.Tags[Family]]
	z        goke.OptComp[world.Z]
	selected tag.Tag[Family]
}

var _ render.Source = (*Renderer)(nil)

// NewRenderer builds a Renderer with DefaultHighlightStyle, outlining what look draws.
func NewRenderer(selected tag.Tag[Family], look func() world.Look) *Renderer {
	return &Renderer{style: DefaultHighlightStyle(), selected: selected, look: look}
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
	look := r.look()
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
				r.footprint = look.Footprint(cam, bases[i].Pos.AABB.AABB, alt, r.footprint[:0])
				r.style.Compose(f, r.footprint)
			}
		}
	}
	if r.marquees != nil {
		r.marquees.compose(f, cam)
	}
}
