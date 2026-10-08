package ui

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
)

// Button is a label that gives its commands when clicked, in order: the scene's own (Show, Hide,
// Toggle), or any other the way a key gives it (Scene.Issue).
func Button(label string, cmds ...any) *Element {
	b := newElement(&button{cmds: cmds}, Label(label)).Padding(6)
	b.style, b.stroke = buttonStyle, 1
	return b
}

type button struct{ cmds []any }

func (*button) place(e *Element, box geom.AABB) { e.children[0].lay(box) }

func (*button) draw(*Element, *render.Image) {}

func (*button) needs(e *Element) (w, h float64) { return e.children[0].needs() }

// Input is whoever takes the input over a picture of the world — a player — told by the active
// scene, before each pass of input, where on the screen the picture lies and what it shows (a
// render.Feed, whose camera ui never looks at).
type Input interface {
	Over(area geom.AABB, shown render.Surface)
}

// Input hands the input over the picture to in: a click on it reaches in, nothing of ui keeps it.
func (e *Element) Input(in Input) *Element {
	if p, ok := e.content.(*picture); ok {
		p.input = in
	}
	return e
}
