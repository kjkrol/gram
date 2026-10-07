package ui

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
)

const (
	panelPadding = 8
	panelStroke  = 2
)

// Panel is a background and a border round its element, padded.
func Panel(e *Element) *Element {
	p := newElement(layers{}, e).Padding(panelPadding)
	p.style, p.stroke = panelStyle, panelStroke
	return p
}

// Label is a line of text, or lines of it, one under another.
func Label(text string) *Element { return newElement(&label{text: text}) }

type label struct{ text string }

func (*label) place(*Element, geom.AABB) {}

func (l *label) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	th := e.look()
	x, y := float32(math.Round(box.TopLeft.X)), float32(math.Round(box.TopLeft.Y))
	if th.Shadow.A > 0 {
		render.DrawText(dst, th.Font, l.text, x+1, y+1, th.Shadow)
	}
	render.DrawText(dst, th.Font, l.text, x, y, th.Text)
}

func (l *label) needs(e *Element) (w, h float64) {
	tw, th := e.look().Font.Measure(l.text)
	return math.Ceil(float64(tw)), math.Ceil(float64(th))
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

// direct is a surface that draws straight onto the screen it fills (render.Feed.DrawOn).
type direct interface{ DrawOn(dst *render.Image) }

func (p *picture) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	if d, ok := p.src.(direct); ok && e.mask == nil && fills(box, dst) {
		d.DrawOn(dst)
		return
	}
	img := p.src.Draw()
	if img == nil {
		return
	}
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

// fills reports whether box covers the whole of dst.
func fills(box geom.AABB, dst *render.Image) bool {
	b := dst.Bounds()
	return math.Round(box.TopLeft.X) <= float64(b.Min.X) && math.Round(box.TopLeft.Y) <= float64(b.Min.Y) &&
		math.Round(box.BottomRight.X) >= float64(b.Max.X) && math.Round(box.BottomRight.Y) >= float64(b.Max.Y)
}

// Window is a panel with a title over its elements, one under another, each as large as it needs.
func Window(title string, elements ...*Element) *Element {
	t := Label(title).Padding(4)
	t.style = titleStyle
	parts := []Part{Fit(t)}
	for _, e := range elements {
		parts = append(parts, Fit(e))
	}
	w := newElement(&window{}, Rows(parts...)).Padding(panelPadding)
	w.style, w.stroke = panelStyle, panelStroke
	return w
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
