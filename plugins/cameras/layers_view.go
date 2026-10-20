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
// a part of its frame, where each side of it fading inside the frame stops fading.
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
	c := cam.(camera.Cropping)
	win := c.Window()
	fw, fh := c.Frame()
	left, right := win.X > 0, win.X+win.W < fw
	top, bottom := win.Y > 0, win.Y+win.H < fh
	w, h := cam.Viewport()
	// the feather and the line's width in shown pixels, drawn in the camera's
	k := camera.ResolutionOf(cam)
	in := func(fades bool) float32 {
		if fades {
			return float32(render.LayerFeather) * k
		}
		return 0
	}
	x0, y0, x1, y1 := in(left), in(top), w-in(right), h-in(bottom)
	corners := [4][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	at := render.OnTop(layersLevel)
	for i, fades := range [4]bool{top, right, bottom, left} {
		if fades {
			a, b := corners[i], corners[(i+1)%4]
			f.Line(at, a[0], a[1], b[0], b[1], 2*k, layerColor)
		}
	}
}
