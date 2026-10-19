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
// corners it turns from, a translucent yellow, or hides them (Settings.Zones). Shift+H by
// default, in turn.
type Zones struct{ Shown bool }

func (Zones) Command() {}

var _ command.Command = Zones{}

// Settings is what the cameras let the players adjust, saved with the game.
type Settings struct {
	Zones    bool
	Adaptive bool
	Rings    bool
}

// Resources is what the cameras publish: their Settings.
type Resources struct {
	Settings Settings
}

var (
	zoneColor   = color.RGBA{R: 255, G: 220, B: 60, A: 10}
	cornerColor = color.RGBA{R: 255, G: 220, B: 60, A: 10}
)

// pixels: the gap between a strip and the corners beside it, and the rounding of every zone
const (
	zoneGap   = 6
	zoneRound = 10
)

// the zones' level among the marks: over every other, the selection's box included
const zonesLevel = 3

// zonesView lays the scroll zones over the picture of the camera being composed.
type zonesView struct{ settings *Settings }

var _ render.Source = (*zonesView)(nil)

func (*zonesView) Init(*goke.SysInit) {}

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

// draw fills the zone in c with its corners rounded by zoneRound, a mark on top
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
	f.Fan(render.OnTop(zonesLevel), pts, c)
}

// ShowZones shows the scroll zones over every picture, or hides them (Settings.Zones).
func (p *Plugin) ShowZones(shown bool) { p.Res.Settings.Zones = shown }

// ZonesShown reports whether the scroll zones are shown.
func (p *Plugin) ZonesShown() bool { return p.Res.Settings.Zones }

// SetAdaptive has every picture's resolution follow the frame budget, or stay
// (Settings.Adaptive).
func (p *Plugin) SetAdaptive(on bool) {
	p.Res.Settings.Adaptive = on
	p.adapter.on = on
}

// Adaptive reports whether the pictures' resolution follows the frame budget.
func (p *Plugin) Adaptive() bool { return p.Res.Settings.Adaptive }

// SetRings has every picture but a minimap's drawn in rings, the whole frame at 0.6 of its
// pixels, the central 60% at 0.8 and the central 35% at all, or whole at the resolution it has
// (Settings.Rings).
func (p *Plugin) SetRings(on bool) {
	p.Res.Settings.Rings = on
	for _, cam := range p.cameras {
		p.ring(cam)
	}
}

// Rings reports whether the pictures are drawn in rings.
func (p *Plugin) Rings() bool { return p.Res.Settings.Rings }

// ring has cam drawn as Settings.Rings says, a minimap's as it is
func (p *Plugin) ring(cam camera.Camera) {
	c, ok := cam.(camera.Ringed)
	if !ok || fitsWhole(cam) {
		return
	}
	if !p.Res.Settings.Rings {
		c.SetRings(nil)
		return
	}
	c.SetRings(defaultRings)
	if r, ok := cam.(camera.Resolved); ok {
		r.SetResolution(1)
	}
}

func (p *Plugin) switchZones() {
	p.zones.Drain(func(i command.Issued[Zones]) { p.ShowZones(i.Command.Shown) })
}

// Settings are the scroll zones, the resolution following the frame budget and the picture
// drawn in rings (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{
		control.Switch{
			Label: "Scroll zones",
			Get:   p.ZonesShown,
			Set:   func(on bool) command.Command { return Zones{Shown: on} },
		},
		control.Switch{
			Label: "Resolution follows the frame budget",
			Get:   p.Adaptive,
			Set:   func(on bool) command.Command { return Adapt{On: on, Rings: p.Rings()} },
		},
		control.Switch{
			Label: "Picture drawn in rings",
			Get:   p.Rings,
			Set:   func(on bool) command.Command { return Adapt{On: p.Adaptive(), Rings: on} },
		},
	}
}
