package atmosphere

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// the telemetry panel's look: its see-through fill and a ui.Panel's padding
var panelFill = color.RGBA{R: 20, G: 22, B: 28, A: 150}

const panelPadding = 8

// TimeOfDay is the hour of the game's day, "08:15", on a see-through panel for a scene to place.
func (p *Plugin) TimeOfDay() *ui.Element { return panel(p.hour()) }

// Date is the season and the date, "summer, 3 August", on a see-through panel, the moon in its
// phase beside them, a line tall, while the moon runs (Running.Moon).
func (p *Plugin) Date() *ui.Element { return panel(p.date()) }

// Weather is the weather, the temperature and the wind, "clear · 29°C · wind 13 E", on a
// see-through panel; left out while none of the climate's workings run.
func (p *Plugin) Weather() *ui.Element {
	w := &weatherWords{p: p}
	return ui.Layers(ui.Drawer(w), panel(ui.LabelOf(w).Steady())).Passing()
}

// Bar is the hour, the date and the weather side by side on one see-through panel, for a scene
// to place at the top (ui.TopMiddle); the weather left out as Weather is.
func (p *Plugin) Bar() *ui.Element {
	w := &weatherWords{p: p}
	// a panel's padding round each: as wide as the three panels side by side
	return ui.Layers(ui.Drawer(w), ui.Columns(
		ui.Fit(p.hour().Margin(panelPadding)),
		ui.Fit(p.date().Margin(panelPadding)),
		ui.Fit(ui.LabelOf(w).Steady().Margin(panelPadding)),
	).Fill(panelFill)).Passing()
}

// a Panel would stay as an empty box round words left out; the pointer passes through to the
// picture under, as the cameras' zones are drawn over it
func panel(e *ui.Element) *ui.Element {
	return e.Fill(panelFill).Padding(panelPadding).Passing()
}

func (p *Plugin) hour() *ui.Element { return ui.LabelOf(&hourWords{p: p}).Steady() }

func (p *Plugin) date() *ui.Element {
	line := render.DefaultFont().LineHeight()
	moon := &moonIcon{moon: func() float32 { return p.drawn().moon }}
	// the disc at the right end of its box, 6 pixels off the words
	icon := ui.Image(moon).Size(line+6, line)
	return ui.Columns(
		ui.Fit(ui.LabelOf(&dateWords{p: p, icon: icon}).Steady()),
		// an anchor takes no room while its element is hidden
		ui.Fit(ui.MiddleRight(icon)),
	)
}

// words built anew once the clock has moved or the weather named is another, not every frame
type framed struct {
	at    time.Duration
	named string
	text  string
	read  bool
}

func (f *framed) of(p *Plugin, named string, say func() string) string {
	if t := p.worldPlugin.Read(0).Clock().Time(); !f.read || t != f.at || named != f.named {
		f.text, f.at, f.named, f.read = say(), t, named, true
	}
	return f.text
}

type hourWords struct {
	p *Plugin
	framed
}

func (h *hourWords) Text(uid.UID64, bool) (string, bool) {
	return h.of(h.p, "", func() string { return h.p.calendar.Now().Hour() }), true
}

type dateWords struct {
	p    *Plugin
	icon *ui.Element
	framed
}

// the icon is shown or hidden here, its words read before it is laid
func (d *dateWords) Text(uid.UID64, bool) (string, bool) {
	if d.p.read.Running().Moon {
		d.icon.Show()
	} else {
		d.icon.Hide()
	}
	return d.of(d.p, "", func() string {
		m := d.p.calendar.Now()
		return fmt.Sprintf("%s, %s", m.Season(), m.Written())
	}), true
}

// weatherWords read the weather as the climate's report does; drawn as a ui.Drawer, it builds
// its query
type weatherWords struct {
	p     *Plugin
	query *goke.Query
	now   goke.Comp[climate.Weather]
	framed
}

func (w *weatherWords) Init(si *goke.SysInit) { w.query = si.NewQueryBuilder(&w.now).Build() }

func (*weatherWords) Draw(*render.Image) {}

func (w *weatherWords) Text(uid.UID64, bool) (string, bool) {
	if w.query == nil || w.p.climate.Running() == (climate.Running{}) {
		return "", false
	}
	text := w.of(w.p, w.p.climate.Now(), w.say)
	return text, text != ""
}

func (w *weatherWords) say() string {
	for w.query.All(); w.query.Next(); {
		now := w.now.At(w.query.Cursor())
		if now.Lanes[0].State < 0 {
			return "" // before the weather's first step
		}
		name := w.name(now.Lanes[0].State)
		if here := w.name(now.Over); here != name {
			name += " (here " + here + ")"
		}
		if now.Snow > now.Rain && now.Snow > 0.05 {
			name += " (snow)"
		}
		return fmt.Sprintf("%s · %.0f°C · wind %.0f %s", name, now.Temperature, now.Blow,
			compass(now.Heading))
	}
	return ""
}

func (w *weatherWords) name(st int32) string {
	if all := w.p.climate.Config().Weathers; st >= 0 && int(st) < len(all) {
		return all[st].Name
	}
	return "?"
}

// heading in radians from +x, east, with y running south
func compass(heading float32) string {
	ways := [...]string{"E", "SE", "S", "SW", "W", "NW", "N", "NE"}
	at := int(math.Round(float64(heading)/(math.Pi/4))) % 8
	if at < 0 {
		at += 8
	}
	return ways[at]
}

// moonIcon is the moon in its phase as the north sees it, lit on the right while waxing: a disc
// across the shorter side of its box at the box's right end, drawn twice as fine as shown for
// its edge to come out smooth
type moonIcon struct {
	moon  func() float32
	face  *render.Image
	img   *render.Image
	w, h  int
	drawn float32
}

var _ render.Surface = (*moonIcon)(nil)

func (m *moonIcon) Resize(w, h int) { m.w, m.h = w, h }

func (m *moonIcon) Draw() *render.Image {
	if m.w <= 0 || m.h <= 0 {
		return nil
	}
	if b := m.img; b == nil || b.Bounds().Dx() != 2*m.w || b.Bounds().Dy() != 2*m.h {
		if b != nil {
			b.Deallocate()
		}
		m.img, m.drawn = render.NewImage(2*m.w, 2*m.h), -1
	}
	// a 512th of the round moves the terminator a tenth of a pixel
	at := float32(math.Round(float64(m.moon())*512) / 512)
	if at != m.drawn {
		if m.face == nil {
			m.face = celestial.MoonFace()
		}
		m.img.Clear()
		drawMoon(m.img, m.face, at)
		m.drawn = at
	}
	return m.img
}

var (
	moonShade = color.RGBA{R: 8, G: 10, B: 16, A: 210}
	moonRim   = color.RGBA{R: 110, G: 115, B: 125, A: 140}
)

// drawMoon draws face as the moon moon round from new (0.5 full) at the right end of dst, its
// part in the shade darkened
func drawMoon(dst, face *render.Image, moon float32) {
	b := dst.Bounds()
	side := float32(min(b.Dx(), b.Dy()))
	r := side/2 - 1
	cx, cy := float32(b.Dx())-side/2, float32(b.Dy())/2
	op := render.DrawImageOptions{Filter: render.FilterLinear}
	k := float64(2*r) / float64(face.Bounds().Dx())
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(cx-r), float64(cy-r))
	lit := float32(1)
	if moon > 0.5 {
		lit = -1
	}
	// 1 new, -1 full: how far the terminator bulges from the middle, towards the smaller part
	bulge := float32(math.Cos(2 * math.Pi * float64(moon)))
	dst.DrawImageIn(face, &op, lune(cx, cy, r, 1, 1))
	if bulge > 0 {
		dst.FillPolygon(lune(cx, cy, r, -lit, bulge), moonShade)
	} else {
		dst.FillPolygon(lune(cx, cy, r, 1, 1), moonShade)
		dst.DrawImageIn(face, &op, lune(cx, cy, r, lit, -bulge))
	}
	dst.StrokePolygon(lune(cx, cy, r, 1, 1), 2, moonRim)
}

// lune is the disc's half on side (1 the right, -1 the left) with the half of an ellipse, bulge
// of the radius wide, on the other: convex, the whole disc at a bulge of 1
func lune(cx, cy, r, side, bulge float32) [][2]float32 {
	const n = 24
	pts := make([][2]float32, 0, 2*n)
	for k := 0; k <= n; k++ {
		s, c := math.Sincos(math.Pi * (float64(k)/n - 0.5))
		pts = append(pts, [2]float32{cx + side*r*float32(c), cy + r*float32(s)})
	}
	for k := n - 1; k > 0; k-- {
		s, c := math.Sincos(math.Pi * (float64(k)/n - 0.5))
		pts = append(pts, [2]float32{cx - side*bulge*r*float32(c), cy + r*float32(s)})
	}
	return pts
}
