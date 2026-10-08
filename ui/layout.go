package ui

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
)

// Layers lays every element over the whole box, from the bottom up: each covers those before it.
func Layers(elements ...*Element) *Element { return newElement(layers{}, elements...) }

type layers struct{}

func (layers) place(e *Element, box geom.AABB) {
	for _, c := range e.children {
		c.lay(box)
	}
}

func (layers) draw(*Element, *render.Image) {}

func (layers) needs(e *Element) (w, h float64) {
	for _, c := range e.children {
		cw, ch := c.needs()
		w, h = max(w, cw), max(h, ch)
	}
	return w, h
}

// Part is an element's part of a split: a share of what is left, fixed pixels, or what it needs.
type Part struct {
	element *Element
	share   float64
	fixed   float64
	fit     bool
}

// Share is n parts of what the fixed and fitted leave: two Share(1, …) are halves, Share(2, …)
// beside Share(1, …) two thirds.
func Share(n float64, e *Element) Part { return Part{element: e, share: n} }

// Fixed is px pixels.
func Fixed(px float64, e *Element) Part { return Part{element: e, fixed: px} }

// Fit is what the element needs: its Size, else its content's.
func Fit(e *Element) Part { return Part{element: e, fit: true} }

// Columns splits the box into columns, left to right, one a part; none covers another.
func Columns(parts ...Part) *Element { return split(true, parts) }

// Rows splits the box into rows, top to bottom, one a part; none covers another.
func Rows(parts ...Part) *Element { return split(false, parts) }

func split(across bool, parts []Part) *Element {
	children := make([]*Element, len(parts))
	for i, p := range parts {
		children[i] = p.element
	}
	return newElement(&splitting{across: across, parts: parts}, children...)
}

// splitting lays its parts side by side (across) or one under another.
type splitting struct {
	across bool
	parts  []Part
}

func (s *splitting) place(e *Element, box geom.AABB) {
	w, h := size(box)
	room := h
	if s.across {
		room = w
	}
	lengths := make([]float64, len(s.parts))
	shares := 0.0
	for i, p := range s.parts {
		switch {
		case p.fit:
			pw, ph := p.element.needs()
			lengths[i] = ph
			if s.across {
				lengths[i] = pw
			}
		case p.share > 0:
			shares += p.share
			continue
		default:
			lengths[i] = p.fixed
		}
		room -= lengths[i]
	}
	for i, p := range s.parts {
		if p.share > 0 && shares > 0 {
			lengths[i] = max(room, 0) * p.share / shares
		}
	}
	at := box.TopLeft
	for i, p := range s.parts {
		if s.across {
			p.element.lay(geom.NewAABBAt(at, lengths[i], h))
			at.X += lengths[i]
		} else {
			p.element.lay(geom.NewAABBAt(at, w, lengths[i]))
			at.Y += lengths[i]
		}
	}
}

func (*splitting) draw(*Element, *render.Image) {}

func (s *splitting) needs(e *Element) (w, h float64) {
	for _, c := range e.children {
		cw, ch := c.needs()
		if s.across {
			w, h = w+cw, max(h, ch)
		} else {
			w, h = max(w, cw), h+ch
		}
	}
	return w, h
}

// anchoring places its one element at a point of the box — a corner, the middle of a side, the
// middle — at the size it asks for (Size on the anchor, else the element's own).
type anchoring struct{ x, y float64 } // 0 left/top, 0.5 middle, 1 right/bottom

func anchor(x, y float64, e *Element) *Element { return newElement(anchoring{x, y}, e) }

func (a anchoring) place(e *Element, box geom.AABB) {
	c := e.children[0]
	cw, ch := c.needs()
	if e.w > 0 && e.h > 0 {
		cw, ch = e.w, e.h
	}
	w, h := size(box)
	switch {
	case e.fw > 0 || e.fh > 0:
		cw, ch = e.share(w, h, c)
	case c.fw > 0 || c.fh > 0:
		cw, ch = c.share(w, h, c)
	}
	at := box.TopLeft.Add(geom.NewVec((w-cw)*a.x, (h-ch)*a.y))
	c.lay(geom.NewAABBAt(at, cw, ch))
}

// share is the size of c where e asks for a Fraction of a w by h box: an axis asked for as 0
// following the proportions of the picture c shows, else c's own size; a picture with proportions
// asked for both keeps them within.
func (e *Element) share(w, h float64, c *Element) (float64, float64) {
	sw, sh := e.fw*w, e.fh*h
	if pw, ph, ok := c.proportions(); ok && pw > 0 && ph > 0 {
		switch {
		case sw > 0 && sh > 0:
			k := min(sw/pw, sh/ph)
			return pw * k, ph * k
		case sw > 0:
			return sw, sw * ph / pw
		default:
			return sh * pw / ph, sh
		}
	}
	nw, nh := c.needs()
	if sw == 0 {
		sw = nw
	}
	if sh == 0 {
		sh = nh
	}
	return sw, sh
}

// proportions are the width and height of what the element's picture shows, where it says them
// (render.Feed.Proportions).
func (e *Element) proportions() (w, h float64, ok bool) {
	p, is := e.content.(*picture)
	if !is {
		return 0, 0, false
	}
	if s, says := p.src.(interface {
		Proportions() (float64, float64, bool)
	}); says {
		return s.Proportions()
	}
	return 0, 0, false
}

func (anchoring) draw(*Element, *render.Image) {}

func (anchoring) needs(e *Element) (w, h float64) { return e.children[0].needs() }

// The anchors: each places its element at its point of the box, kept Margin pixels from the edges.
func TopLeft(e *Element) *Element      { return anchor(0, 0, e) }
func TopMiddle(e *Element) *Element    { return anchor(0.5, 0, e) }
func TopRight(e *Element) *Element     { return anchor(1, 0, e) }
func MiddleLeft(e *Element) *Element   { return anchor(0, 0.5, e) }
func Center(e *Element) *Element       { return anchor(0.5, 0.5, e) }
func MiddleRight(e *Element) *Element  { return anchor(1, 0.5, e) }
func BottomLeft(e *Element) *Element   { return anchor(0, 1, e) }
func BottomMiddle(e *Element) *Element { return anchor(0.5, 1, e) }
func BottomRight(e *Element) *Element  { return anchor(1, 1, e) }

func (layers) container()     {}
func (*splitting) container() {}
func (anchoring) container()  {}
