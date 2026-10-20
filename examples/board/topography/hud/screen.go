// Package hud lays out the demo's screen.
package hud

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// Deps is what the screen is made of.
type Deps struct {
	// Feed is the world through the player's camera, Minimap the minimap round the same camera
	// and Compass its north, hung on its edge.
	Feed       *ui.Element
	Minimap    *ui.Element
	Compass    *ui.Element
	Clock      render.Renderer
	Paused     func() bool
	Atmosphere *atmosphere.Plugin
	// Reason says why a bridge cannot stand where it is aimed, Window asks to build it.
	Reason *ui.Element
	Window *ui.Element
	// Widgets lays the hour, the date and the weather apart by the corners, not in a bar.
	Widgets bool
}

var (
	reasonColor = color.RGBA{R: 20, G: 20, B: 20, A: 180}
	pausedColor = color.RGBA{R: 120, G: 30, B: 30, A: 200}
	minimapRim  = color.RGBA{R: 30, G: 30, B: 30, A: 255}
)

const (
	minimapSide = 220
	minimapBox  = minimapSide + 2*minimap.Overhang // the compass hangs past the circle
)

// Screen is the feed under the clock, the minimap top-right, the atmosphere's hour, date and
// weather, PAUSED beside them while the clock stands, and the bridges' words over all; the
// top-left corner is the telemetry panel's.
func Screen(d Deps) *ui.Element {
	return ui.Layers(
		d.Feed,
		ui.Drawer(d.Clock),
		ui.TopRight(ui.Layers(
			ui.Center(d.Minimap.Masked(ui.Circle).Border(minimapRim, 2)).Size(minimapSide, minimapSide),
			d.Compass,
		)).Size(minimapBox, minimapBox).Margin(16-minimap.Overhang),
		timeAndWeather(d),
		ui.BottomMiddle(d.Reason.Padding(6).Fill(reasonColor)).Margin(40),
		ui.Center(d.Window),
	)
}

// the bar at the top, or apart: the hour over the clock, the date at the top, the weather
// under the minimap; PAUSED beside the bar or the date
func timeAndWeather(d Deps) *ui.Element {
	a := d.Atmosphere
	if !d.Widgets {
		return ui.TopMiddle(beside(a.Bar(), d.Paused)).Margin(8)
	}
	return ui.Layers(
		// the clock's line is 20 pixels over the bottom
		ui.BottomLeft(ui.Rows(ui.Fit(a.TimeOfDay()), room(20))).Margin(8),
		ui.TopMiddle(beside(a.Date(), d.Paused)).Margin(8),
		ui.TopRight(ui.Rows(room(minimapSide+8), ui.Fit(a.Weather()))).Margin(16),
	)
}

const (
	pausedPad = 6 // round the word
	pausedGap = 8 // between the word and the element it stands beside
)

// beside is e with PAUSED on a red panel to its right while is says so: a slot as wide is kept
// on either side, so e stays centred as the word comes and goes
func beside(e *ui.Element, is func() bool) *ui.Element {
	w, _ := ui.DefaultTheme().Bold.Measure("PAUSED")
	slot := float64(w) + 2*pausedPad + pausedGap
	word := ui.HeadingOf(ui.While(is, "PAUSED")).Padding(pausedPad).Fill(pausedColor)
	return ui.Columns(room(slot), ui.Fit(e), ui.Fixed(slot, ui.MiddleRight(word)))
}

// room is a strip px tall, or wide, that, unlike a Blank, takes no click
func room(px float64) ui.Part { return ui.Fixed(px, ui.Layers()) }
