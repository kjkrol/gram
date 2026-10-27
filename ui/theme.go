package ui

import (
	"image/color"

	"github.com/kjkrol/gram/render"
)

// Theme is how a scene's elements look where they say nothing of their own: the font and the colour
// of their text, and the colours of panels, windows' titles, buttons and the widgets' marks.
type Theme struct {
	Font   *render.Font
	Bold   *render.Font // a Heading's; nil, the Font
	Large  *render.Font // a Title's; nil, the Bold
	Text   color.RGBA
	Shadow color.RGBA // a pixel under and right of the text; zero for none
	Panel  color.RGBA // a panel's and a window's background
	Border color.RGBA // a panel's, a window's and a button's border
	Title  color.RGBA // a window's title's background
	Button color.RGBA // a button's background
	Accent color.RGBA // a checkbox's tick, a slider's knob and the track behind it

	dim *Theme // the theme as a dead element wears it, made once
}

// dimmed is the theme with its text, its borders and the widgets' marks at half their alpha: a
// dead element's
func (th *Theme) dimmed() *Theme {
	if th.dim == nil {
		d := *th
		d.Text.A, d.Border.A, d.Accent.A = d.Text.A/2, d.Border.A/2, d.Accent.A/2
		th.dim = &d
	}
	return th.dim
}

// DefaultTheme is light text in Go Regular over dark panels.
func DefaultTheme() Theme {
	return Theme{
		Font:   render.DefaultFont(),
		Bold:   render.DefaultBoldFont(),
		Large:  render.DefaultLargeFont(),
		Text:   color.RGBA{R: 240, G: 240, B: 240, A: 255},
		Shadow: color.RGBA{A: 200},
		Panel:  color.RGBA{R: 20, G: 22, B: 28, A: 220},
		Border: color.RGBA{R: 200, G: 200, B: 210, A: 255},
		Title:  color.RGBA{R: 50, G: 56, B: 72, A: 255},
		Button: color.RGBA{R: 60, G: 70, B: 95, A: 255},
		Accent: color.RGBA{R: 120, G: 170, B: 255, A: 255},
	}
}

type style uint8

const (
	plain style = iota
	panelStyle
	titleStyle
	buttonStyle
)

var fallback = DefaultTheme()

func (e *Element) look() *Theme {
	th := e.theme
	if th == nil {
		th = &fallback
	}
	if e.dead() {
		return th.dimmed()
	}
	return th
}

func (e *Element) fillColor() color.RGBA {
	if e.filled {
		return e.fill
	}
	th := e.look()
	switch e.style {
	case panelStyle:
		return th.Panel
	case titleStyle:
		return th.Title
	case buttonStyle:
		return th.Button
	}
	return color.RGBA{}
}

func (e *Element) borderColor() color.RGBA {
	if e.bordered || e.style == plain || e.style == titleStyle {
		return e.border
	}
	return e.look().Border
}

// Theme draws the scene's elements in t where they say nothing of their own.
func (s *Scene) Theme(t Theme) *Scene {
	s.theme = t
	s.dress()
	return s
}

func (s *Scene) dress() {
	if s.root == nil {
		return
	}
	s.root.walk(func(e *Element) {
		e.theme = &s.theme
		if e.pin != nil && e.pin.show != nil {
			e.pin.show.walk(func(b *Element) { b.theme = &s.theme })
		}
	})
	if s.overlay != nil {
		s.overlay.walk(func(e *Element) { e.theme = &s.theme })
	}
}
