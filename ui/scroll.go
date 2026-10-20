package ui

import (
	"image"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Scroll shows e, as wide and as tall as it needs, through the box it is given: what lies past
// the box's bottom comes into it scrolled, by the bar at the box's right edge dragged or by the
// wheel over any of it; an e that fits shows whole, the bar not drawn.
func Scroll(e *Element) *Element { return newElement(&scroll{}, e) }

type scroll struct {
	offset   float64 // the pixels of the content scrolled past the top
	tall     float64 // the content's height
	dragging bool
	grip     float64 // how far into the thumb the drag took hold
}

// pixels: the bar's width and the thumb's least length
const (
	scrollBar   = 12
	scrollThumb = 24
	scrollStep  = 48 // a notch of the wheel
)

var _ pressable = (*scroll)(nil)
var _ dragged = (*scroll)(nil)

func (sc *scroll) needs(e *Element) (w, h float64) {
	w, h = e.children[0].needs()
	return w + scrollBar, h
}

func (sc *scroll) place(e *Element, box geom.AABB) {
	_, h := size(box)
	_, sc.tall = e.children[0].needs()
	sc.offset = sc.clamped(sc.offset, h)
	top := box.TopLeft.Y - sc.offset
	e.children[0].lay(geom.NewAABB(geom.NewVec(box.TopLeft.X, top),
		geom.NewVec(box.BottomRight.X-scrollBar, top+math.Max(sc.tall, h))))
}

func (sc *scroll) clamped(offset, shown float64) float64 {
	return math.Max(0, math.Min(offset, sc.tall-shown))
}

// clip is the part of dst the content is drawn into: the box less the bar
func (sc *scroll) clip(e *Element, dst *render.Image) *render.Image {
	box := shrink(e.box, e.padding)
	return dst.SubImage(image.Rect(int(math.Floor(box.TopLeft.X)), int(math.Floor(box.TopLeft.Y)),
		int(math.Ceil(box.BottomRight.X-scrollBar)), int(math.Ceil(box.BottomRight.Y))))
}

func (sc *scroll) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	_, h := size(box)
	if sc.tall <= h {
		return
	}
	th := e.look()
	x := float32(box.BottomRight.X - scrollBar/2)
	render.StrokeLine(dst, x, float32(box.TopLeft.Y), x, float32(box.BottomRight.Y), 2, th.Border)
	y0, y1 := sc.thumb(box)
	render.StrokeLine(dst, x, float32(y0), x, float32(y1), scrollBar-4, th.Accent)
}

// thumb is where the bar's thumb lies: its share of the bar the box's of the content
func (sc *scroll) thumb(box geom.AABB) (y0, y1 float64) {
	_, h := size(box)
	span := math.Max(scrollThumb, h*h/sc.tall)
	y0 = box.TopLeft.Y + sc.offset/(sc.tall-h)*(h-span)
	return y0, y0 + span
}

// onBar reports whether at lies on the bar
func (sc *scroll) onBar(e *Element, at geom.Vec) bool {
	box := shrink(e.box, e.padding)
	return at.X >= box.BottomRight.X-scrollBar
}

func (sc *scroll) press(s *Scene, e *Element, at geom.Vec, _ uid.UID64, _ bool) {
	box := shrink(e.box, e.padding)
	if _, h := size(box); sc.tall <= h || !sc.onBar(e, at) {
		return
	}
	y0, y1 := sc.thumb(box)
	sc.grip = (y1 - y0) / 2
	if at.Y >= y0 && at.Y < y1 {
		sc.grip = at.Y - y0
	}
	sc.dragging = true
	s.held = e
	sc.drag(s, e, at)
}

func (sc *scroll) drag(_ *Scene, e *Element, at geom.Vec) {
	box := shrink(e.box, e.padding)
	_, h := size(box)
	y0, y1 := sc.thumb(box)
	if track := h - (y1 - y0); track > 0 {
		sc.offset = sc.clamped((at.Y-sc.grip-box.TopLeft.Y)/track*(sc.tall-h), h)
	}
}

func (sc *scroll) drop() { sc.dragging = false }

// wheel scrolls a notch: down for a wheel rolled down (a delta below zero)
func (sc *scroll) wheel(_ *Scene, e *Element, delta float64) {
	_, h := size(shrink(e.box, e.padding))
	sc.offset = sc.clamped(sc.offset-math.Copysign(scrollStep, delta), h)
}
