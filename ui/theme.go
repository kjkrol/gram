package ui

import (
	"image/color"

	"github.com/kjkrol/gram/render"
)

// Theme is how a scene's elements look where they say nothing of their own: the font and the colour
// of their text, and the colours of panels, windows' titles and buttons.
type Theme struct {
	Font   *render.Font
	Text   color.RGBA
	Shadow color.RGBA // a pixel under and right of the text; zero for none
	Panel  color.RGBA // a panel's and a window's background
	Border color.RGBA // a panel's, a window's and a button's border
	Title  color.RGBA // a window's title's background
	Button color.RGBA // a button's background
}

// DefaultTheme is light text in Go Regular over dark panels.
func DefaultTheme() Theme {
	return Theme{
		Font:   render.DefaultFont(),
		Text:   color.RGBA{R: 240, G: 240, B: 240, A: 255},
		Shadow: color.RGBA{A: 200},
		Panel:  color.RGBA{R: 20, G: 22, B: 28, A: 220},
		Border: color.RGBA{R: 200, G: 200, B: 210, A: 255},
		Title:  color.RGBA{R: 50, G: 56, B: 72, A: 255},
		Button: color.RGBA{R: 60, G: 70, B: 95, A: 255},
	}
}

// style is which of the theme's colours an element takes where it sets none of its own.
type style uint8

const (
	plain style = iota
	panelStyle
	titleStyle
	buttonStyle
)

var fallback = DefaultTheme()

// look is the theme the element is drawn in: its scene's, else the default.
func (e *Element) look() *Theme {
	if e.theme != nil {
		return e.theme
	}
	return &fallback
}

// fillColor is the element's background: its own Fill, else its style's in the theme.
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

// borderColor is the element's border: its own Border, else the theme's for a styled element.
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

// dress hands every element of the scene its theme.
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
}
