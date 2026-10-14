package world

import (
	"image/color"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var boxColor = color.RGBA{R: 230, G: 40, B: 40, A: 255}

const boxWidth = 1

// boxesView outlines every entity's box in sight of the camera, over everything.
type boxesView struct {
	p     *Plugin
	shown bool
	q     *goke.Query
	base  goke.Comp[Base]
	quads []camera.Quad
}

func (v *boxesView) Init(si *goke.SysInit) { v.q = si.NewQueryBuilder(&v.base).Build() }

func (v *boxesView) Compose(f *render.Frame, cam camera.Camera) {
	if !v.shown {
		return
	}
	in := v.p.ViewFor(cam)
	for v.q.All(); v.q.Next(); {
		cur := v.q.Cursor()
		for i, id := range cur.IDs {
			if in != nil && !in.Contains(id) {
				continue
			}
			b := v.base.Slice(cur)[i].Pos.AABB
			v.quads = cam.ToScreenQuads(float32(b.TopLeft.X), float32(b.TopLeft.Y),
				float32(b.BottomRight.X), float32(b.BottomRight.Y), v.quads[:0])
			for _, q := range v.quads {
				f.Line(render.Marks, 0, q.X0, q.Y0, q.X1, q.Y0, boxWidth, boxColor)
				f.Line(render.Marks, 0, q.X1, q.Y0, q.X1, q.Y1, boxWidth, boxColor)
				f.Line(render.Marks, 0, q.X1, q.Y1, q.X0, q.Y1, boxWidth, boxColor)
				f.Line(render.Marks, 0, q.X0, q.Y1, q.X0, q.Y0, boxWidth, boxColor)
			}
		}
	}
}

// ShowBoxes outlines every entity's box in red over the picture, or hides the outlines: a look
// for finding out why things touch or miss, not saved. BoxesRenderer must be in the composer.
func (p *Plugin) ShowBoxes(shown bool) { p.boxesView.shown = shown }

// BoxesShown reports whether the entities' boxes are outlined.
func (p *Plugin) BoxesShown() bool { return p.boxesView.shown }

// BoxesRenderer is the layer outlining the entities' boxes while shown, for a scene's composer.
func (p *Plugin) BoxesRenderer() render.Layer { return p.boxesView }
