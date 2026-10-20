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
// weather, PAUSED under them while the clock stands, and the bridges' words over all; the
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
// under the minimap; PAUSED under the bar or the date
func timeAndWeather(d Deps) *ui.Element {
	a := d.Atmosphere
	if !d.Widgets {
		return ui.TopMiddle(ui.Rows(ui.Fit(a.Bar()), ui.Fit(paused(d.Paused)))).Margin(8)
	}
	return ui.Layers(
		// the clock's line is 20 pixels over the bottom
		ui.BottomLeft(ui.Rows(ui.Fit(a.TimeOfDay()), room(20))).Margin(8),
		ui.TopMiddle(ui.Rows(ui.Fit(a.Date()), ui.Fit(paused(d.Paused)))).Margin(8),
		ui.TopRight(ui.Rows(room(minimapSide+8), ui.Fit(a.Weather()))).Margin(16),
	)
}

// paused is PAUSED on a red panel while is says so, else nothing, centred in its row
func paused(is func() bool) *ui.Element {
	return ui.Center(ui.HeadingOf(ui.While(is, "PAUSED")).Padding(6).Fill(pausedColor).Margin(4))
}

// room is a strip px tall that, unlike a Blank, takes no click
func room(px float64) ui.Part { return ui.Fixed(px, ui.Layers()) }
