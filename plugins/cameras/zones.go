package cameras

import (
	"image/color"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Zones shows the strips at the picture's edges the cursor scrolls from and the squares at its
// corners it turns from, a translucent yellow deepening to the edge as the scroll quickens, or
// hides them (Settings.Zones). The settings screen alone.
type Zones struct{ Shown bool }

// EdgeScroll has the cursor at a picture's edge scroll the camera, or not (Settings.StillEdges).
// The settings screen alone.
type EdgeScroll struct{ On bool }

// EdgeSpeed sets how fast the cursor at a picture's edge scrolls, screen pixels a tick at the
// very edge (Settings.EdgeSpeed); 0, the keys' own. The settings screen alone.
type EdgeSpeed struct{ Speed float32 }

func (Zones) Command()      {}
func (EdgeScroll) Command() {}
func (EdgeSpeed) Command()  {}

var (
	_ command.Command = Zones{}
	_ command.Command = EdgeScroll{}
	_ command.Command = EdgeSpeed{}
)

// a zone's colour at the picture's edge, and clear at its inner bound
var (
	zoneColor = color.RGBA{R: 255, G: 220, B: 60, A: 40}
	zoneClear = color.RGBA{}
)

// pixels: the gap between a strip and the corners beside it
const zoneGap = 6

// zonesView draws the scroll zones on the glass over every picture but one of the whole world:
// in shown pixels, over whatever the scene lays over the picture (render.Glass).
type zonesView struct{ drawnSettings }

var _ render.Source = (*zonesView)(nil)
var _ render.Glass = (*zonesView)(nil)

// Compose composes nothing: the zones go on the glass.
func (*zonesView) Compose(*render.Frame, camera.Camera) {}

// DrawGlass draws the zones over dst, the picture as shown: the strips control.EdgeMargin wide,
// the corners control.CornerSide a side, each clear inside and zoneColor at the edge.
func (v *zonesView) DrawGlass(dst *render.Image, cam camera.Camera) {
	if !v.drawn().Zones || fitsWhole(cam) {
		return
	}
	b := dst.Bounds()
	w, h := float32(b.Dx()), float32(b.Dy())
	thick := float32(control.EdgeMargin)
	side := float32(control.CornerSide(float64(w), float64(h)))
	ends := side + zoneGap
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
		ox, oy := float32(b.Min.X), float32(b.Min.Y)
		dst.FillQuad(render.Corners{{ox + x0, oy + y0}, {ox + x1, oy + y0},
			{ox + x0, oy + y1}, {ox + x1, oy + y1}},
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

func (p *Plugin) switchZones() {
	p.zones.Drain(func(i command.Issued[Zones]) { p.write.ShowZones(i.Command.Shown) })
	p.edgeScrolls.Drain(func(i command.Issued[EdgeScroll]) { p.write.ScrollFromEdges(i.Command.On) })
	p.edgeSpeeds.Drain(func(i command.Issued[EdgeSpeed]) { p.write.SetEdgeSpeed(i.Command.Speed) })
}
