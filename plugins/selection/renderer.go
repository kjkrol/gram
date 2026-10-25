package selection

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Highlight is how a Selected entity of a kind is outlined: its footprint, the ground under it on
// screen, stroked in Line, Width pixels wide, a mark on top (render.OnTop). A figure declares it
// on the world's atlas (Slot.Styled); a kind without one is outlined by DefaultHighlight.
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
	at := render.OnTop(outlineLevel)
	for _, c := range footprint {
		outline(f, at, [4][2]float32{c[0], c[1], c[3], c[2]}, h.Width, h.Line)
	}
}

func outline(f *render.Frame, at render.At, pts [4][2]float32, width float32, c color.RGBA) {
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		f.Line(at, p[0], p[1], q[0], q[1], width, c)
	}
}

// the marks' levels: the box being dragged over the outlines
const (
	outlineLevel = 1
	marqueeLevel = 2
)

// Renderer outlines every Selected entity and, when the plugin built it, the box being dragged in
// the viewport's camera, as published once a frame.
type Renderer struct {
	marquees  *marquees
	look      func() world.Look
	styles    func() *render.Styles
	footprint []render.Corners
	drawn     render.Buffered[outlined]

	query    *goke.Query
	base     goke.Comp[world.Base]
	marks    goke.Comp[tag.Tags[Family]]
	app      goke.OptComp[render.Appearance]
	z        goke.OptComp[world.Z]
	selected tag.Tag[Family]
}

var _ render.Source = (*Renderer)(nil)
var _ render.Overlying = (*Renderer)(nil)

// outlined is what the Renderer draws: every Selected entity's box, height and Highlight, and the
// boxes being dragged
type outlined struct {
	all      []highlit
	marquees marquees
}

type highlit struct {
	box geom.AABB
	alt float32
	h   Highlight
}

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

// Overlies: drawn over the figures' ground (render.Overlying).
func (*Renderer) Overlies() {}

// Publish takes every Selected entity and the boxes being dragged, for the frame drawn next.
func (r *Renderer) Publish() {
	b := r.drawn.Back()
	b.all = b.all[:0]
	var styles *render.Styles
	if r.styles != nil {
		styles = r.styles()
	}
	for r.query.All(); r.query.Next(); {
		cursor := r.query.Cursor()
		bases, marks := r.base.Slice(cursor), r.marks.Slice(cursor)
		apps, zs := r.app.Slice(cursor), r.z.Slice(cursor)
		for i := range cursor.IDs {
			if !marks[i].Has(r.selected) {
				continue
			}
			e := highlit{box: bases[i].Pos.AABB.AABB, h: DefaultHighlight}
			if zs != nil {
				e.alt = float32(zs[i].Altitude)
			}
			if apps != nil {
				if st, ok := render.StyleOf[Highlight](styles, apps[i].SpriteID); ok {
					e.h = st
				}
			}
			b.all = append(b.all, e)
		}
	}
	if r.marquees != nil {
		r.marquees.copyInto(&b.marquees)
	}
}

// Swap has what was published last drawn.
func (r *Renderer) Swap() { r.drawn.Swap() }

// Compose outlines the Selected entities through cam, and the box being dragged in it, marks on
// top.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	look := r.look()
	d := r.drawn.Front()
	for _, e := range d.all {
		r.footprint = look.Footprint(cam, e.box, e.alt, r.footprint[:0])
		e.h.compose(f, r.footprint)
	}
	d.marquees.compose(f, cam)
}
