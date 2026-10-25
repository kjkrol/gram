package ui

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Button is a label that gives its commands when clicked, in order: the scene's own (Show, Hide,
// Toggle), or any other the way a key gives it (Scene.Issue).
func Button(label string, cmds ...command.Command) *Element { return newButton(Label(label), cmds) }

// ButtonOf is a Button whose label t says, every frame; with no label it is left out.
func ButtonOf(t Text, cmds ...command.Command) *Element {
	return newButton(LabelOf(t), cmds)
}

func newButton(label *Element, cmds []command.Command) *Element {
	b := newElement(&button{cmds: cmds}, label).Padding(6)
	b.style, b.stroke = buttonStyle, 1
	return b
}

type button struct{ cmds []command.Command }

func (*button) absent(e *Element) bool { return e.children[0].absent() }

func (*button) place(e *Element, box geom.AABB) { e.children[0].lay(box) }

func (*button) draw(*Element, *render.Image) {}

func (*button) needs(e *Element) (w, h float32) { return e.children[0].needs() }

func (b *button) press(s *Scene, _ *Element, _ geom.Vec, about uid.UID64, pinned bool) {
	for _, cmd := range b.cmds {
		s.giveAbout(cmd, about, pinned)
	}
}

// Input is whoever takes the input over a picture of the world, told before each pass of input
// where the picture lies on the screen and what it shows.
type Input interface {
	Over(area geom.AABB, shown render.Surface)
}

// Clicked is an Input that takes a press on its picture itself: Click hears where, in the
// picture's pixels from its top-left corner, and neither the press nor the wheel over the picture
// goes further.
type Clicked interface {
	Input
	Click(at geom.Vec)
}

// Hovered is an Input told before each pass of input whether the cursor lies on its picture, no
// element over it there.
type Hovered interface {
	Input
	Hover(on bool)
}

// Input hands the input over the picture to in: a click on it reaches in, nothing of ui keeps it.
func (e *Element) Input(in Input) *Element {
	if p, ok := e.content.(*picture); ok {
		p.input = in
	}
	return e
}
