package ui

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// pressable is a content taking a press on its element itself: a button, a checkbox, a slider's
// knob, a dropdown's field and its options.
type pressable interface {
	press(s *Scene, e *Element, at geom.Vec, about uid.UID64, pinned bool)
}

// Checkbox is a box with label beside it, ticked while get says so, read every frame; a click
// gives the command set builds for the other state. It reads no entity: pinned, every instance
// shows the same.
func Checkbox(label string, get func() bool, set func(on bool) command.Command) *Element {
	return newElement(&checkbox{get: get, set: set}, Label(label))
}

type checkbox struct {
	get func() bool
	set func(on bool) command.Command
}

const checkGap = 6

func (c *checkbox) side(e *Element) float32 {
	_, h := e.children[0].needs()
	return h
}

func (c *checkbox) place(e *Element, box geom.AABB) {
	off := geom.NewVec(c.side(e)+checkGap, 0)
	e.children[0].lay(geom.NewAABB(box.TopLeft.Add(off), box.BottomRight))
}

func (c *checkbox) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	side := c.side(e)
	_, h := size(box)
	x, y := box.TopLeft.X, box.TopLeft.Y+(h-side)/2
	s := side
	th := e.look()
	// half a pixel in: a one-pixel stroke on whole pixels
	dst.StrokeRect(x+0.5, y+0.5, s-1, s-1, 1, th.Border)
	if c.ticked(e) {
		dst.StrokeLine(x+s*0.2, y+s*0.5, x+s*0.45, y+s*0.8, 2, th.Accent)
		dst.StrokeLine(x+s*0.45, y+s*0.8, x+s*0.85, y+s*0.2, 2, th.Accent)
	}
}

// ticked is the box's tick as published, read now before
func (c *checkbox) ticked(e *Element) bool {
	if f := e.front(); f != nil {
		if on, ok := f.ticks[c]; ok {
			return on
		}
	}
	return c.get()
}

func (c *checkbox) needs(e *Element) (w, h float32) {
	w, h = e.children[0].needs()
	return w + c.side(e) + checkGap, h
}

func (c *checkbox) press(s *Scene, _ *Element, _ geom.Vec, about uid.UID64, pinned bool) {
	s.giveAbout(c.set(!c.get()), about, pinned)
}

// Slider is a knob on a track, at the number get says between min and max; dragged, or stepped
// by the wheel over it, it gives the command set builds for each new number, a step apart.
// Size sets its length; it reads no entity.
func Slider(min, max, step float64, get func() float64,
	set func(v float64) command.Command) *Element {
	return newElement(&slider{min: min, max: max, step: step, get: get, set: set})
}

type slider struct {
	min, max, step float64
	get            func() float64
	set            func(v float64) command.Command
	dragging       bool
	value          float64
	about          uid.UID64
	pinned         bool
}

const (
	sliderLength = 160
	sliderHeight = 18
)

func (*slider) place(*Element, geom.AABB) {}

func (sl *slider) needs(*Element) (w, h float32) { return sliderLength, sliderHeight }

// shown is the number the knob stands at: dragged, its own, else as published, read now before
func (sl *slider) shown(e *Element) float64 {
	if sl.dragging {
		return sl.value
	}
	if f := e.front(); f != nil {
		if v, ok := f.values[sl]; ok {
			return v
		}
	}
	return sl.get()
}

func (sl *slider) at(box geom.AABB, v float64) float32 {
	w, h := size(box)
	r := h / 2
	if sl.max <= sl.min {
		return box.TopLeft.X + r
	}
	f := float32(min(1, max(0, (v-sl.min)/(sl.max-sl.min))))
	return box.TopLeft.X + r + f*(w-2*r)
}

func (sl *slider) valueAt(box geom.AABB, x float32) float64 {
	w, h := size(box)
	r := h / 2
	f := float64((x - box.TopLeft.X - r) / (w - 2*r))
	v := sl.min + min(1, max(0, f))*(sl.max-sl.min)
	return sl.quantized(v)
}

func (sl *slider) quantized(v float64) float64 {
	if sl.step > 0 {
		v = sl.min + math.Round((v-sl.min)/sl.step)*sl.step
	}
	return min(sl.max, max(sl.min, v))
}

func (sl *slider) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	w, h := size(box)
	th := e.look()
	left, right := box.TopLeft.X+h/2, box.TopLeft.X+w-h/2
	mid := box.TopLeft.Y + h/2
	knob := sl.at(box, sl.shown(e))
	dst.StrokeLine(left, mid, right, mid, 2, th.Border)
	dst.StrokeLine(left, mid, knob, mid, 2, th.Accent)
	dst.FillCircle(knob, mid, h/2, th.Accent)
}

func (sl *slider) press(s *Scene, e *Element, at geom.Vec, about uid.UID64, pinned bool) {
	sl.dragging, sl.about, sl.pinned = true, about, pinned
	sl.value = sl.get()
	s.held = e
	sl.drag(s, e, at)
}

func (sl *slider) drag(s *Scene, e *Element, at geom.Vec) {
	v := sl.valueAt(shrink(e.box, e.padding), at.X)
	if v != sl.value {
		sl.value = v
		s.giveAbout(sl.set(v), sl.about, sl.pinned)
	}
}

func (sl *slider) drop() { sl.dragging = false }

func (sl *slider) wheel(s *Scene, e *Element, delta float32) {
	step := sl.step
	if step == 0 {
		step = (sl.max - sl.min) / 20
	}
	v := sl.quantized(sl.get() + math.Copysign(step, float64(delta)))
	if v != sl.get() {
		s.giveAbout(sl.set(v), e.of, e.ofPinned)
	}
}

// Dropdown is a field showing the option get chooses, read every frame; clicked, it opens the
// options over everything, and the one clicked gives the command set builds for its index.
// It reads no entity.
func Dropdown(options []string, get func() int, set func(i int) command.Command) *Element {
	d := &dropdown{options: options, get: get, set: set}
	e := newElement(d, LabelOf(chosen{d})).Padding(6)
	e.style, e.stroke = buttonStyle, 1
	return e
}

type dropdown struct {
	options []string
	get     func() int
	set     func(i int) command.Command
	about   uid.UID64
	pinned  bool
}

type chosen struct{ d *dropdown }

func (c chosen) Text(uid.UID64, bool) (string, bool) {
	i := c.d.get()
	if i < 0 || i >= len(c.d.options) {
		return "", true
	}
	return c.d.options[i], true
}

const arrowRoom = 16

func (d *dropdown) place(e *Element, box geom.AABB) {
	e.children[0].lay(geom.NewAABB(box.TopLeft, box.BottomRight.Sub(geom.NewVec(arrowRoom, 0))))
}

func (d *dropdown) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	_, h := size(box)
	x, y := box.BottomRight.X-arrowRoom+3, box.TopLeft.Y+h/2-3
	dst.FillPolygon([][2]float32{{x, y}, {x + 10, y}, {x + 5, y + 6}}, e.look().Text)
}

func (d *dropdown) needs(e *Element) (w, h float32) {
	l := e.children[0].content.(*label)
	font := l.font(e.look())
	for _, o := range d.options {
		ow, oh := font.Measure(o)
		w, h = max(w, f32.Ceil(ow)), max(h, f32.Ceil(oh))
	}
	return w + arrowRoom, h
}

func (d *dropdown) press(s *Scene, e *Element, _ geom.Vec, about uid.UID64, pinned bool) {
	d.about, d.pinned = about, pinned
	parts := make([]Part, len(d.options))
	for i, o := range d.options {
		item := newElement(&option{d: d, i: i}, Label(o)).Padding(6)
		item.style, item.stroke = buttonStyle, 0
		parts[i] = Fit(item)
	}
	s.open(Panel(Rows(parts...)).Padding(2), e.box)
}

type option struct {
	d *dropdown
	i int
}

func (*option) place(e *Element, box geom.AABB) { e.children[0].lay(box) }

func (*option) draw(*Element, *render.Image) {}

func (*option) needs(e *Element) (w, h float32) { return e.children[0].needs() }

func (o *option) press(s *Scene, _ *Element, _ geom.Vec, _ uid.UID64, _ bool) {
	s.close()
	s.giveAbout(o.d.set(o.i), o.d.about, o.d.pinned)
}
