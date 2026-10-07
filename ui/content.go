package ui

import (
	"image/color"
	"math"
	"strings"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
)

// The look of a panel and a window until a theme says otherwise.
var (
	panelFill   = color.RGBA{R: 20, G: 22, B: 28, A: 220}
	panelBorder = color.RGBA{R: 200, G: 200, B: 210, A: 255}
	titleFill   = color.RGBA{R: 50, G: 56, B: 72, A: 255}
)

const (
	panelPadding = 8
	panelStroke  = 2
	lineHeight   = 16 // a line of render's text
)

// Panel is a background and a border round its element, padded.
func Panel(e *Element) *Element {
	return newElement(layers{}, e).Fill(panelFill).Border(panelBorder, panelStroke).Padding(panelPadding)
}

// Label is a line of text, or lines of it, one under another.
func Label(text string) *Element { return newElement(&label{text: text}) }

type label struct{ text string }

func (*label) place(*Element, geom.AABB) {}

func (l *label) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	render.DebugPrintAt(dst, l.text, int(math.Round(box.TopLeft.X)), int(math.Round(box.TopLeft.Y)))
}

func (l *label) needs(*Element) (w, h float64) {
	return float64(render.TextWidth(l.text)), float64(lineHeight * (strings.Count(l.text, "\n") + 1))
}

// Image shows src filling the element's box, the box's size given to it: a feed of the world, a
// picture.
func Image(src render.Surface) *Element { return newElement(&picture{src: src}) }

type picture struct {
	src   render.Surface
	input Input // who takes the input over it; nil: ui keeps a click on it
}

func (p *picture) place(e *Element, box geom.AABB) {
	w, h := size(box)
	p.src.Resize(int(math.Round(w)), int(math.Round(h)))
	if p.input != nil {
		p.input.Over(box)
	}
}

func (p *picture) draw(e *Element, dst *render.Image) {
	img := p.src.Draw()
	if img == nil {
		return
	}
	box := shrink(e.box, e.padding)
	x, y := float32(math.Round(box.TopLeft.X)), float32(math.Round(box.TopLeft.Y))
	if e.mask != nil {
		dst.DrawImageIn(img, x, y, e.mask.points(box))
		return
	}
	op := &render.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(img, op)
}

func (*picture) needs(*Element) (w, h float64) { return 0, 0 }

// Window is a panel with a title over its elements, one under another, each as large as it needs.
func Window(title string, elements ...*Element) *Element {
	parts := []Part{Fit(Label(title).Fill(titleFill).Padding(4))}
	for _, e := range elements {
		parts = append(parts, Fit(e))
	}
	return newElement(&window{}, Rows(parts...)).Fill(panelFill).Border(panelBorder, panelStroke).Padding(panelPadding)
}

// window is a panel with a title.
type window struct{}

func (*window) place(e *Element, box geom.AABB) { e.children[0].lay(box) }

func (*window) draw(*Element, *render.Image) {}

func (*window) needs(e *Element) (w, h float64) { return e.children[0].needs() }

// Blank is an element showing nothing of its own: a gap, or with a Fill a divider, a plate.
func Blank() *Element { return newElement(blank{}) }

type blank struct{}

func (blank) place(*Element, geom.AABB)     {}
func (blank) draw(*Element, *render.Image)  {}
func (blank) needs(*Element) (w, h float64) { return 0, 0 }

// Layer is an element drawn by r, a screen layer drawing itself in the screen's pixels — a
// telemetry line, a background of its own; the scene initialises it once.
func Layer(r render.Renderer) *Element { return newElement(&layered{r: r}) }

type layered struct{ r render.Renderer }

func (*layered) place(*Element, geom.AABB)            {}
func (l *layered) draw(_ *Element, dst *render.Image) { l.r.Draw(dst) }
func (*layered) needs(*Element) (w, h float64)        { return 0, 0 }
func (*layered) container()                           {} // hit by nothing: it draws, it takes no click
