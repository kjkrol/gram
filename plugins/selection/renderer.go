package selection

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// HighlightStyle draws one Selected entity's outline, given its world-space AABB and the altitude
// it stands at (0 in a flat world).
type HighlightStyle interface {
	Draw(screen *ebiten.Image, cam camera.Camera, box camera.AABB, altitude float32)
}

// HighlightStyleFn adapts a plain function to HighlightStyle.
type HighlightStyleFn func(screen *ebiten.Image, cam camera.Camera, box camera.AABB, altitude float32)

func (f HighlightStyleFn) Draw(screen *ebiten.Image, cam camera.Camera, box camera.AABB, altitude float32) {
	f(screen, cam, box, altitude)
}

var _ HighlightStyle = HighlightStyleFn(nil)

var highlightColor = color.RGBA{R: 220, G: 40, B: 40, A: 255}

// DefaultHighlightStyle draws a thin red outline around box; through an isometric camera the box
// is the diamond on the ground under the entity.
func DefaultHighlightStyle() HighlightStyle {
	return HighlightStyleFn(func(screen *ebiten.Image, cam camera.Camera, box camera.AABB, altitude float32) {
		x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
		x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
		if _, iso := cam.Projection().(camera.Isometric); iso {
			c := render.ProjectCorners(cam, x0, y0, x1, y1, altitude)
			var path vector.Path
			path.MoveTo(c[0][0], c[0][1])
			path.LineTo(c[1][0], c[1][1])
			path.LineTo(c[3][0], c[3][1])
			path.LineTo(c[2][0], c[2][1])
			path.Close()
			var cs ebiten.ColorScale
			cs.ScaleWithColor(highlightColor)
			vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: 2}, &vector.DrawPathOptions{ColorScale: cs, AntiAlias: true})
			return
		}
		var buf [4]camera.Quad
		for _, q := range cam.ToScreenQuads(x0, y0, x1, y1, buf[:0]) {
			vector.StrokeRect(screen, q.X0, q.Y0, q.X1-q.X0, q.Y1-q.Y0, 2, highlightColor, true)
		}
	})
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

var _ render.WorldRenderer = (*Renderer)(nil)

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

// DrawWorld outlines the Selected entities through cam.
func (r *Renderer) DrawWorld(screen *ebiten.Image, cam camera.Camera) {
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
				r.style.Draw(screen, cam, bases[i].Pos.AABB.AABB, alt)
			}
		}
	}
	if r.marquees != nil {
		r.marquees.draw(screen, cam)
	}
}
