package cameras

import (
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var layerColor = color.RGBA{R: 80, G: 220, B: 255, A: 200}

// the layers' level among the marks: over the zones
const layersLevel = 4

// layersView outlines every inner layer of the camera being composed over its picture, where
// each side of it fading inside the frame stops fading; composed once in the frame's pixels, a
// picture drawn a layer at a time sees the outlines in each layer's own pixels (camera.InFrame).
type layersView struct{ settings *Settings }

var _ render.Source = (*layersView)(nil)
var _ render.Overlying = (*layersView)(nil)

func (*layersView) Init(*goke.SysInit) {}

// Overlies: drawn over the figures' ground (render.Overlying).
func (*layersView) Overlies() {}

func (v *layersView) Compose(f *render.Frame, cam camera.Camera) {
	layers := camera.LayersOf(cam)
	c, crops := cam.(camera.Cropping)
	if !v.settings.ShowLayers || len(layers) < 2 || !crops {
		return
	}
	fw, fh := c.Frame()
	wins := camera.Arrange(camera.LayoutOf(cam), layers, int(math.Round(float64(fw))),
		int(math.Round(float64(fh))))
	// the line's width in shown pixels, drawn in the camera's
	k := camera.ResolutionOf(cam)
	at := render.OnTop(layersLevel)
	in := func(fades bool) float32 {
		if fades {
			return render.LayerFeather
		}
		return 0
	}
	for _, win := range wins[1:] {
		left, right := win.X > 0, win.X+win.W < fw
		top, bottom := win.Y > 0, win.Y+win.H < fh
		x0, y0 := camera.InFrame(cam, win.X+in(left), win.Y+in(top))
		x1, y1 := camera.InFrame(cam, win.X+win.W-in(right), win.Y+win.H-in(bottom))
		corners := [4][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
		for i, fades := range [4]bool{top, right, bottom, left} {
			if fades {
				a, b := corners[i], corners[(i+1)%4]
				f.Line(at, a[0], a[1], b[0], b[1], 2*k, layerColor)
			}
		}
	}
}
