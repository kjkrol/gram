package cameras

import (
	"image/color"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Zones shows the strips at the picture's edges the cursor scrolls from and the squares at its
// corners it turns from, a translucent yellow deepening to the edge as the scroll quickens, or
// hides them (Settings.Zones). The settings screen alone.
type Zones struct{ Shown bool }

func (Zones) Command() {}

var _ command.Command = Zones{}

// a zone's colour at the picture's edge, and clear at its inner bound
var (
	zoneColor = color.RGBA{R: 255, G: 220, B: 60, A: 40}
	zoneClear = color.RGBA{}
)

// pixels: the gap between a strip and the corners beside it
const zoneGap = 6

// the zones' level among the marks: over every other, the selection's box included
const zonesLevel = 3

// zonesView lays the scroll zones over the picture of the camera being composed.
type zonesView struct{ settings *Settings }

var _ render.Source = (*zonesView)(nil)
var _ render.Overlying = (*zonesView)(nil)

func (*zonesView) Init(*goke.SysInit) {}

// Overlies: drawn over the figures' ground (render.Overlying).
func (*zonesView) Overlies() {}

func (v *zonesView) Compose(f *render.Frame, cam camera.Camera) {
	if !v.settings.Zones || camera.IsCropped(cam) {
		return
	}
	w, h := cam.Viewport()
	// the zones in shown pixels, drawn in the camera's: how far in a strip reaches, a corner's
	// square, where a strip stops from an edge
	k := camera.ResolutionOf(cam)
	thick := float32(control.EdgeMargin) * k
	side := float32(control.CornerSide(float64(w/k), float64(h/k))) * k
	ends := side + zoneGap*k

	// each zone's corners top-left, top-right, bottom-left, bottom-right, coloured where they lie
	// on the picture's edge
	zone := func(x0, y0, x1, y1 float32) {
		if x1 <= x0 || y1 <= y0 {
			return
		}
		at := func(x, y float32) color.RGBA {
			if x == 0 || x == w || y == 0 || y == h {
				return zoneColor
			}
			return zoneClear
		}
		f.Quad(render.OnTop(zonesLevel), render.Corners{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}},
			[4]color.RGBA{at(x0, y0), at(x1, y0), at(x0, y1), at(x1, y1)})
	}
	zone(ends, 0, w-ends, thick)
	zone(ends, h-thick, w-ends, h)
	zone(0, ends, thick, h-ends)
	zone(w-thick, ends, w, h-ends)

	zone(0, 0, side, side)
	zone(w-side, 0, w, side)
	zone(0, h-side, side, h)
	zone(w-side, h-side, w, h)
}

// ShowZones shows the scroll zones over every picture, or hides them (Settings.Zones).
func (p *Plugin) ShowZones(shown bool) { p.Res.Settings.Zones = shown }

// ZonesShown reports whether the scroll zones are shown.
func (p *Plugin) ZonesShown() bool { return p.Res.Settings.Zones }

func (p *Plugin) switchZones() {
	p.zones.Drain(func(i command.Issued[Zones]) { p.ShowZones(i.Command.Shown) })
}
