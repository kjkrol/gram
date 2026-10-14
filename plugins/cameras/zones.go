package cameras

import (
	"image/color"
	"math"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Zones shows the strips at the picture's edges the cursor scrolls from and the squares at its
// corners it turns from, a translucent yellow, or hides them: a look, not saved. Shift+H by
// default.
type Zones struct{}

func (Zones) Command() {}

var _ command.Command = Zones{}

var (
	zoneColor   = color.RGBA{R: 255, G: 220, B: 60, A: 10}
	cornerColor = color.RGBA{R: 255, G: 220, B: 60, A: 10}
)

// pixels: the gap between a strip and the corners beside it, and the rounding of every zone
const (
	zoneGap   = 6
	zoneRound = 10
)

// zonesView lays the scroll zones over the picture of the camera being composed.
type zonesView struct{ shown bool }

var _ render.Source = (*zonesView)(nil)

func (*zonesView) Init(*goke.SysInit) {}

func (v *zonesView) Compose(f *render.Frame, cam camera.Camera) {
	if !v.shown {
		return
	}
	w, h := cam.Viewport()
	m, g := float32(control.EdgeMargin), float32(zoneGap)
	r := float32(control.CornerSide(float64(w), float64(h)))
	rounded(f, r+g, 0, w-r-g, m, zoneColor)
	rounded(f, r+g, h-m, w-r-g, h, zoneColor)
	rounded(f, 0, r+g, m, h-r-g, zoneColor)
	rounded(f, w-m, r+g, w, h-r-g, zoneColor)
	for _, c := range [4][2]float32{{0, 0}, {w - r, 0}, {0, h - r}, {w - r, h - r}} {
		rounded(f, c[0], c[1], c[0]+r, c[1]+r, cornerColor)
	}
}

// a rectangle with its corners rounded by zoneRound, on the Marks tier
func rounded(f *render.Frame, x0, y0, x1, y1 float32, c color.RGBA) {
	if x1 <= x0 || y1 <= y0 {
		return
	}
	rad := min(float32(zoneRound), (x1-x0)/2, (y1-y0)/2)
	const arc = 20 // segments a corner
	var pts [][2]float32
	centres := [4][2]float32{
		{x1 - rad, y0 + rad}, {x1 - rad, y1 - rad}, {x0 + rad, y1 - rad}, {x0 + rad, y0 + rad},
	}
	for k, centre := range centres {
		for i := 0; i <= arc; i++ {
			a := (float64(k) + float64(i)/arc - 1) * math.Pi / 2
			x, y := centre[0]+rad*float32(math.Cos(a)), centre[1]+rad*float32(math.Sin(a))
			pts = append(pts, [2]float32{x, y})
		}
	}
	f.Fan(render.Marks, 0, pts, c)
}

// ShowZones shows the scroll zones over every picture, or hides them.
func (p *Plugin) ShowZones(shown bool) { p.zonesView.shown = shown }

// ZonesShown reports whether the scroll zones are shown.
func (p *Plugin) ZonesShown() bool { return p.zonesView.shown }

func (p *Plugin) switchZones() {
	p.zones.Drain(func(command.Issued[Zones]) { p.zonesView.shown = !p.zonesView.shown })
}
