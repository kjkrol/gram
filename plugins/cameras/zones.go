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
	thick := float32(control.EdgeMargin)                        // how far in a strip reaches
	side := float32(control.CornerSide(float64(w), float64(h))) // a corner's square
	ends := side + zoneGap                                      // where a strip stops, from an edge

	topStrip := zone{left: ends, top: 0, right: w - ends, bottom: thick}
	bottomStrip := zone{left: ends, top: h - thick, right: w - ends, bottom: h}
	leftStrip := zone{left: 0, top: ends, right: thick, bottom: h - ends}
	rightStrip := zone{left: w - thick, top: ends, right: w, bottom: h - ends}

	topLeftCorner := zone{left: 0, top: 0, right: side, bottom: side}
	topRightCorner := zone{left: w - side, top: 0, right: w, bottom: side}
	bottomLeftCorner := zone{left: 0, top: h - side, right: side, bottom: h}
	bottomRightCorner := zone{left: w - side, top: h - side, right: w, bottom: h}

	for _, strip := range []zone{topStrip, bottomStrip, leftStrip, rightStrip} {
		strip.draw(f, zoneColor)
	}
	corners := []zone{topLeftCorner, topRightCorner, bottomLeftCorner, bottomRightCorner}
	for _, corner := range corners {
		corner.draw(f, cornerColor)
	}
}

// a rectangle of the picture, in its pixels, y down the screen
type zone struct{ left, top, right, bottom float32 }

// draw fills the zone in c with its corners rounded by zoneRound, on the Marks tier
func (z zone) draw(f *render.Frame, c color.RGBA) {
	x0, y0, x1, y1 := z.left, z.top, z.right, z.bottom
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
