package cameras

import (
	"image/color"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var layerColor = color.RGBA{R: 80, G: 220, B: 255, A: 200}

// the layers' level among the marks: over the zones
const layersLevel = 4

// layersView outlines a layer over the picture of the camera being composed, on the pass drawing
// a part of its frame, where the layer's edge stops fading.
type layersView struct{ settings *Settings }

var _ render.Source = (*layersView)(nil)
var _ render.Overlying = (*layersView)(nil)

func (*layersView) Init(*goke.SysInit) {}

// Overlies: drawn over the figures' ground (render.Overlying).
func (*layersView) Overlies() {}

func (v *layersView) Compose(f *render.Frame, cam camera.Camera) {
	if !v.settings.ShowLayers || !camera.IsCropped(cam) {
		return
	}
	w, h := cam.Viewport()
	// the feather and the line's width in shown pixels, drawn in the camera's
	k := camera.ResolutionOf(cam)
	in := float32(render.LayerFeather) * k
	width := 2 * k
	at := render.OnTop(layersLevel)
	f.Line(at, in, in, w-in, in, width, layerColor)
	f.Line(at, w-in, in, w-in, h-in, width, layerColor)
	f.Line(at, w-in, h-in, in, h-in, width, layerColor)
	f.Line(at, in, h-in, in, in, width, layerColor)
}
