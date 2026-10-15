package collision

import (
	"image/color"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Boxes shows every entity's box as a red outline over the picture, or hides them: a look for
// finding out why things touch or miss, not saved. Shift+X by default.
type Boxes struct{}

func (Boxes) Command() {}

var _ command.Command = Boxes{}

// Queues are where Boxes lands.
func (p *Plugin) Queues() []command.CommandQueue { return []command.CommandQueue{&p.boxes} }

// DefaultBindings are Shift+X showing or hiding the entities' boxes.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyX, Mods: control.Mods{Shift: true}},
			"Show every entity's box, or hide them",
			func(control.Context) (Boxes, bool) { return Boxes{}, true }),
	}
}

var boxColor = color.RGBA{R: 230, G: 40, B: 40, A: 255}

const boxWidth = 1

// boxesView outlines every entity's box in sight of the camera, over everything.
type boxesView struct {
	w     *world.Plugin
	shown bool
	q     *goke.Query
	base  goke.Comp[world.Base]
	quads []camera.Quad
}

func (v *boxesView) Init(si *goke.SysInit) { v.q = si.NewQueryBuilder(&v.base).Build() }

func (v *boxesView) Compose(f *render.Frame, cam camera.Camera) {
	if !v.shown {
		return
	}
	in := v.w.ViewFor(cam)
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
				f.Line(render.OnTop, q.X0, q.Y0, q.X1, q.Y0, boxWidth, boxColor)
				f.Line(render.OnTop, q.X1, q.Y0, q.X1, q.Y1, boxWidth, boxColor)
				f.Line(render.OnTop, q.X1, q.Y1, q.X0, q.Y1, boxWidth, boxColor)
				f.Line(render.OnTop, q.X0, q.Y1, q.X0, q.Y0, boxWidth, boxColor)
			}
		}
	}
}

// ShowBoxes outlines every entity's box in red over the picture, or hides the outlines: a look
// for finding out why things touch or miss, not saved. Renderer draws them.
func (p *Plugin) ShowBoxes(shown bool) { p.boxesView.shown = shown }

// BoxesShown reports whether the entities' boxes are outlined.
func (p *Plugin) BoxesShown() bool { return p.boxesView.shown }

func (p *Plugin) switchBoxes() {
	p.boxes.Drain(func(command.Issued[Boxes]) { p.ShowBoxes(!p.BoxesShown()) })
}
