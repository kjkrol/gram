package ui

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
)

var buttonFill = color.RGBA{R: 60, G: 70, B: 95, A: 255}

// Button is a label that gives cmd when clicked: one of the scene's own (Show, Hide, Toggle), or
// any other the way a key gives it (Scene.Issue).
func Button(label string, cmd any) *Element {
	return newElement(&button{cmd: cmd}, Label(label)).Fill(buttonFill).Border(panelBorder, 1).Padding(6)
}

type button struct{ cmd any }

func (*button) place(e *Element, box geom.AABB) { e.children[0].lay(box) }

func (*button) draw(*Element, *render.Image) {}

func (*button) needs(e *Element) (w, h float64) { return e.children[0].needs() }

// Input is whoever takes the input over a picture of the world — a player — told every frame where
// on the screen the picture lies.
type Input interface{ Over(area geom.AABB) }

// Input hands the input over the picture to in: a click on it reaches in, nothing of ui keeps it.
func (e *Element) Input(in Input) *Element {
	if p, ok := e.content.(*picture); ok {
		p.input = in
	}
	return e
}
