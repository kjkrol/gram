// Package hud lays out the demo's screen.
package hud

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// Deps is what the screen is made of.
type Deps struct {
	// Feed is the world through the player's camera, Minimap the minimap round the same camera.
	Feed       *ui.Element
	Minimap    *ui.Element
	Clock      render.Renderer
	Atmosphere *atmosphere.Plugin
	// Reason says why a bridge cannot stand where it is aimed, Window asks to build it.
	Reason *ui.Element
	Window *ui.Element
	// Widgets lays the hour, the date and the weather apart by the corners, not in a bar.
	Widgets bool
}

var (
	reasonColor = color.RGBA{R: 20, G: 20, B: 20, A: 180}
	minimapRim  = color.RGBA{R: 30, G: 30, B: 30, A: 255}
)

const minimapSide = 220

// Screen is the feed under the clock, the minimap top-right, the atmosphere's hour, date and
// weather, and the bridges' words over all; the top-left corner is the telemetry panel's.
func Screen(d Deps) *ui.Element {
	return ui.Layers(
		d.Feed,
		ui.Drawer(d.Clock),
		ui.TopRight(d.Minimap.Masked(ui.Circle).Border(minimapRim, 2)).
			Size(minimapSide, minimapSide).
			Margin(16),
		timeAndWeather(d),
		// over the bar
		ui.BottomMiddle(d.Reason.Padding(6).Fill(reasonColor)).Margin(48),
		ui.Center(d.Window),
	)
}

// the bar at the bottom, clear of the telemetry panel growing down the left, or apart: the hour
// over the clock, the weather under the minimap, the date in the corner below
func timeAndWeather(d Deps) *ui.Element {
	a := d.Atmosphere
	if !d.Widgets {
		return ui.BottomMiddle(a.Bar()).Margin(8)
	}
	return ui.Layers(
		// the clock's line is 20 pixels over the bottom
		ui.BottomLeft(ui.Rows(ui.Fit(a.TimeOfDay()), room(20))).Margin(8),
		ui.TopRight(ui.Rows(room(minimapSide+8), ui.Fit(a.Weather()))).Margin(16),
		ui.BottomRight(a.Date()).Margin(16),
	)
}

// room is a strip px tall that, unlike a Blank, takes no click
func room(px float64) ui.Part { return ui.Fixed(px, ui.Layers()) }
