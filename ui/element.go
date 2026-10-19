package ui

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Element is a part of a scene's screen: given a box by its parent, it places its children in it,
// draws itself and says whether a point of the screen hits it.
type Element struct {
	content  content
	children []*Element
	name     string
	hidden   bool
	holds    bool
	w, h     float64
	fw, fh   float64
	margin   float64
	padding  float64
	fill     color.RGBA
	border   color.RGBA
	stroke   float64
	filled   bool
	bordered bool
	steady   bool
	kept     struct{ w, h float64 }
	style    style
	theme    *Theme
	mask     Mask
	pin      *pin
	parent   geom.AABB
	box      geom.AABB
	of       uid.UID64
	ofPinned bool
}

type content interface {
	place(e *Element, box geom.AABB)
	draw(e *Element, dst *render.Image)
	needs(e *Element) (w, h float64)
}

type container interface{ container() }

type leaving interface{ absent(e *Element) bool }

func (e *Element) absent() bool {
	l, ok := e.content.(leaving)
	return ok && l.absent(e)
}

func (e *Element) gone() bool { return e.hidden || e.absent() }

func (e *Element) drawnFor(id uid.UID64, pinned bool) {
	e.walk(func(c *Element) { c.of, c.ofPinned = id, pinned })
}

func newElement(c content, children ...*Element) *Element {
	return &Element{content: c, children: children}
}

// Named calls the element name, for showing and hiding it (Scene.Show, Scene.Hide).
func (e *Element) Named(name string) *Element {
	e.name = name
	return e
}

// Hidden has the element start hidden: not drawn, hit by nothing, until shown.
func (e *Element) Hidden() *Element {
	e.hidden = true
	return e
}

// Show has the element drawn and hit again.
func (e *Element) Show() { e.hidden = false }

// Hide has the element neither drawn nor hit, as Hidden has it start.
func (e *Element) Hide() { e.hidden = true }

// Size asks for w by h pixels: what an anchor places, what Fit takes.
func (e *Element) Size(w, h float64) *Element {
	e.w, e.h = w, h
	return e
}

// Fraction asks for w and h of the box its parent gives it, so the element keeps its share as
// the window changes. An axis asked for as 0 follows the proportions of the picture it shows,
// else its own size.
func (e *Element) Fraction(w, h float64) *Element {
	e.fw, e.fh = w, h
	return e
}

// Margin keeps the element px pixels inside the box its parent gives it, on every side.
func (e *Element) Margin(px float64) *Element {
	e.margin = px
	return e
}

// Padding keeps the element's children px pixels inside its own box, on every side.
func (e *Element) Padding(px float64) *Element {
	e.padding = px
	return e
}

// Fill paints the element's background c.
func (e *Element) Fill(c color.RGBA) *Element {
	e.fill, e.filled = c, true
	return e
}

// Border outlines the element in c, width pixels wide.
func (e *Element) Border(c color.RGBA, width float64) *Element {
	e.border, e.stroke, e.bordered = c, width, true
	return e
}

// Steady keeps the element from shrinking: it needs the widest and the tallest it has needed so
// far, so a box round words that change does not flicker.
func (e *Element) Steady() *Element {
	e.steady = true
	return e
}

// Masked cuts the element to m: its background, its picture and what hits it.
func (e *Element) Masked(m Mask) *Element {
	e.mask = m
	return e
}

// Modal has the element — a window, or an anchor round one — hold the input while it is shown:
// nothing under it or beside it is clicked.
func (e *Element) Modal() *Element {
	e.holds = true
	return e
}

func (e *Element) modal() bool { return e.holds && !e.gone() }

// Box is where the element was last laid, in the screen's pixels.
func (e *Element) Box() geom.AABB { return e.box }

// Hits reports whether the screen point p hits the element: a container where one of its children
// is hit, any other inside its box, and its mask if it has one.
func (e *Element) Hits(p geom.Vec) bool {
	if e.hidden {
		return false
	}
	if e.pin != nil {
		hit := false
		e.pin.each(e, e.parent, func(*instance) { hit = hit || e.hitsHere(p) })
		return hit
	}
	return e.hitsHere(p)
}

func (e *Element) hitsHere(p geom.Vec) bool {
	if e.absent() {
		return false
	}
	if _, ok := e.content.(container); ok && e.fillColor().A == 0 {
		for _, c := range e.children {
			if c.Hits(p) {
				return true
			}
		}
		return false
	}
	if !inside(e.box, p) {
		return false
	}
	return e.mask == nil || e.mask.contains(e.box, p)
}

func (e *Element) needs() (w, h float64) {
	if e.absent() {
		return 0, 0
	}
	w, h = e.w, e.h
	if w == 0 || h == 0 {
		cw, ch := e.content.needs(e)
		if w == 0 {
			w = cw + 2*e.padding
		}
		if h == 0 {
			h = ch + 2*e.padding
		}
	}
	if e.steady {
		e.kept.w, e.kept.h = max(e.kept.w, w), max(e.kept.h, h)
		w, h = e.kept.w, e.kept.h
	}
	return w + 2*e.margin, h + 2*e.margin
}

func (e *Element) lay(box geom.AABB) {
	e.parent = box
	e.box = shrink(box, e.margin)
	e.content.place(e, shrink(e.box, e.padding))
}

func (e *Element) paint(dst *render.Image) {
	if e.hidden {
		return
	}
	if e.pin == nil {
		e.paintHere(dst)
		return
	}
	e.pin.each(e, e.parent, func(in *instance) {
		e.paintHere(dst)
		paintArrow(dst, in, e.look().Border)
		if in.show {
			e.pin.show.paint(dst)
		}
	})
}

func (e *Element) paintHere(dst *render.Image) {
	if e.absent() {
		return
	}
	e.background(dst)
	e.content.draw(e, dst)
	for _, c := range e.children {
		c.paint(dst)
	}
	e.edge(dst)
}

func (e *Element) background(dst *render.Image) {
	if fill := e.fillColor(); fill.A > 0 {
		render.FillPolygon(dst, outline(e.box, e.mask), fill)
	}
}

func (e *Element) edge(dst *render.Image) {
	if border := e.borderColor(); border.A > 0 && e.stroke > 0 {
		render.StrokePolygon(dst, outline(e.box, e.mask), float32(e.stroke), border)
	}
}

func (e *Element) walk(fn func(*Element)) {
	fn(e)
	for _, c := range e.children {
		c.walk(fn)
	}
}

func inside(box geom.AABB, p geom.Vec) bool {
	return p.X >= box.TopLeft.X && p.X < box.BottomRight.X && p.Y >= box.TopLeft.Y && p.Y < box.BottomRight.Y
}

func size(box geom.AABB) (w, h float64) {
	d := box.BottomRight.Sub(box.TopLeft)
	return d.X, d.Y
}

func shrink(box geom.AABB, by float64) geom.AABB {
	if by == 0 {
		return box
	}
	d := geom.NewVec(by, by)
	return geom.NewAABB(box.TopLeft.Add(d), box.BottomRight.Sub(d))
}
