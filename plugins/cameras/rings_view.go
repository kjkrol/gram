package cameras

import (
	"image/color"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var ringColor = color.RGBA{R: 80, G: 220, B: 255, A: 200}

// the rings' level among the marks: over the zones
const ringsLevel = 4

// ringsView outlines a ring over the picture of the camera being composed, on the pass drawing
// a part of its frame, where the ring's edge stops fading.
type ringsView struct{ settings *Settings }

var _ render.Source = (*ringsView)(nil)
var _ render.Overlying = (*ringsView)(nil)

func (*ringsView) Init(*goke.SysInit) {}

// Overlies: drawn over the figures' ground (render.Overlying).
func (*ringsView) Overlies() {}

func (v *ringsView) Compose(f *render.Frame, cam camera.Camera) {
	if !v.settings.ShowRings || !camera.IsCropped(cam) {
		return
	}
	w, h := cam.Viewport()
	// the feather and the line's width in shown pixels, drawn in the camera's
	k := camera.ResolutionOf(cam)
	in := float32(render.RingFeather) * k
	width := 2 * k
	at := render.OnTop(ringsLevel)
	f.Line(at, in, in, w-in, in, width, ringColor)
	f.Line(at, w-in, in, w-in, h-in, width, ringColor)
	f.Line(at, w-in, h-in, in, h-in, width, ringColor)
	f.Line(at, in, h-in, in, in, width, ringColor)
}
