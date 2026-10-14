package selection

import (
	"image/color"

	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Highlight is how a Selected entity of a kind is outlined: its footprint, the ground under it on
// screen, stroked in Line, Width pixels wide, on the Marks tier. A figure declares it on the
// world's atlas (Slot.Styled); a kind without one is outlined by DefaultHighlight.
type Highlight struct {
	Line  color.RGBA
	Width float32
}

// Style: a figure declares it on the world's atlas (render.Style).
func (Highlight) Style() {}

var _ render.Style = Highlight{}

// DefaultHighlight is a thin red outline.
var DefaultHighlight = Highlight{Line: color.RGBA{R: 220, G: 40, B: 40, A: 255}, Width: 2}

func (h Highlight) compose(f *render.Frame, footprint []render.Corners) {
	for _, c := range footprint {
		outline(f, [4][2]float32{c[0], c[1], c[3], c[2]}, h.Width, h.Line)
	}
}

func outline(f *render.Frame, pts [4][2]float32, width float32, c color.RGBA) {
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		f.Line(render.Marks, 0, p[0], p[1], q[0], q[1], width, c)
	}
}

// Renderer outlines every Selected entity and, when the plugin built it, the box being dragged in
// the viewport's camera.
type Renderer struct {
	marquees  *marquees
	look      func() world.Look
	styles    func() *render.Styles
	footprint []render.Corners

	query    *goke.Query
	base     goke.Comp[world.Base]
	marks    goke.Comp[tag.Tags[Family]]
	app      goke.OptComp[render.Appearance]
	z        goke.OptComp[world.Z]
	selected tag.Tag[Family]
}

var _ render.Source = (*Renderer)(nil)

// NewRenderer builds a Renderer outlining what look draws, each kind as the Highlight styles
// hold for it, DefaultHighlight without; styles may give nil.
func NewRenderer(
	selected tag.Tag[Family], look func() world.Look, styles func() *render.Styles,
) *Renderer {
	return &Renderer{selected: selected, look: look, styles: styles}
}

func (r *Renderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.marks).Optional(&r.app).Optional(&r.z).Build()
}

// Compose outlines the Selected entities through cam, and the box being dragged in it, on the Marks
// tier.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	look := r.look()
	var styles *render.Styles
	if r.styles != nil {
		styles = r.styles()
	}
	r.query.All()
	for r.query.Next() {
		cursor := r.query.Cursor()
		bases := r.base.Slice(cursor)
		marks := r.marks.Slice(cursor)
		apps := r.app.Slice(cursor)
		zs := r.z.Slice(cursor)
		for i := range cursor.IDs {
			if marks[i].Has(r.selected) {
				alt := float32(0)
				if zs != nil {
					alt = float32(zs[i].Altitude)
				}
				r.footprint = look.Footprint(cam, bases[i].Pos.AABB.AABB, alt, r.footprint[:0])
				h := DefaultHighlight
				if apps != nil {
					if st, ok := render.StyleOf[Highlight](styles, apps[i].SpriteID); ok {
						h = st
					}
				}
				h.compose(f, r.footprint)
			}
		}
	}
	if r.marquees != nil {
		r.marquees.compose(f, cam)
	}
}
