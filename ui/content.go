package ui

import (
	"image"
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
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

// Subheading is a Label in the theme's Accent colour: a section's part.
func Subheading(text string) *Element {
	return newElement(&label{text: text, weight: subheading})
}

// Heading is a Label in the theme's bold font: a section's title.
func Heading(text string) *Element { return newElement(&label{text: text, weight: heading}) }

// Title is a Label in the theme's large font: a screen's title.
func Title(text string) *Element { return newElement(&label{text: text, weight: title}) }

type weight int

const (
	regular weight = iota
	subheading
	heading
	title
)

// Text is what an element reads its words off once a frame, as its scene is published, for the
// entity it is pinned to. False leaves the element out: it takes no room and nothing hits it.
type Text interface {
	Text(of uid.UID64, pinned bool) (string, bool)
}

// LabelOf is a Label whose words t says, every frame: a pinned element's for its entity.
func LabelOf(t Text) *Element { return newElement(&label{src: t}) }

// HeadingOf is a Heading whose words t says, every frame, as LabelOf.
func HeadingOf(t Text) *Element { return newElement(&label{src: t, weight: heading}) }

// While is a Text saying text while on says so, and nothing else: an element left out till then.
func While(on func() bool, text string) Text { return while{on: on, text: text} }

type while struct {
	on   func() bool
	text string
}

func (w while) Text(uid.UID64, bool) (string, bool) { return w.text, w.on() }

// Of is a Text read off the component T of the entity an element is pinned to: fn says the
// words. An entity without T, or an element pinned to none, says nothing.
func Of[T any](fn func(v *T) (string, bool)) Text { return &readOff[T]{fn: fn} }

type readOff[T any] struct {
	fn    func(v *T) (string, bool)
	query *goke.Query
	comp  goke.Comp[T]
}

func (r *readOff[T]) bind(si *goke.SysInit) { r.query = si.NewQueryBuilder(&r.comp).Build() }

func (r *readOff[T]) Text(of uid.UID64, pinned bool) (string, bool) {
	if !pinned || r.query == nil || !r.query.Seek(of) {
		return "", false
	}
	return r.fn(r.comp.At(r.query.Cursor()))
}

type bound interface{ bind(si *goke.SysInit) }

type label struct {
	text   string
	src    Text
	weight weight
}

func (l *label) font(th *Theme) *render.Font {
	switch {
	case l.weight == title && th.Large != nil:
		return th.Large
	case l.weight >= heading && th.Bold != nil:
		return th.Bold
	}
	return th.Font
}

// words are the label's as published for the entity it is drawn for: read now for one not yet
// published, none for an entity of a pin's not published
func (l *label) words(e *Element) (string, bool) {
	if l.src == nil {
		return l.text, true
	}
	if f := e.front(); f != nil {
		if w, ok := f.words[saying{l, e.of, e.ofPinned}]; ok {
			return w.text, w.ok
		}
		if e.ofPinned {
			return "", false
		}
	}
	return l.src.Text(e.of, e.ofPinned)
}

func (l *label) absent(e *Element) bool {
	_, ok := l.words(e)
	return !ok
}

func (*label) place(*Element, geom.AABB) {}

func (l *label) draw(e *Element, dst *render.Image) {
	text, _ := l.words(e)
	box := shrink(e.box, e.padding)
	th := e.look()
	x, y := float32(math.Round(box.TopLeft.X)), float32(math.Round(box.TopLeft.Y))
	if th.Shadow.A > 0 {
		l.font(th).Draw(dst, text, x+1, y+1, th.Shadow)
	}
	l.font(th).Draw(dst, text, x, y, l.colour(th))
}

func (l *label) colour(th *Theme) color.RGBA {
	if l.weight == subheading && th.Accent.A > 0 {
		return th.Accent
	}
	return th.Text
}

func (l *label) needs(e *Element) (w, h float64) {
	text, _ := l.words(e)
	tw, th := l.font(e.look()).Measure(text)
	return math.Ceil(float64(tw)), math.Ceil(float64(th))
}

// Image shows src filling the element's box, the box's size given to it: a feed of the world, a
// picture.
func Image(src render.Surface) *Element { return newElement(&picture{src: src}) }

type picture struct {
	src   render.Surface
	input Input
	glass geom.AABB // where the picture was drawn this frame, for its source's glass
	drawn bool
}

// glassed is a picture's source drawing on the glass over it, last of all (render.Feed)
type glassed interface{ DrawGlass(dst *render.Image) }

// place sizes the source to its box, or to the screen it is drawn straight onto
func (p *picture) place(e *Element, box geom.AABB) {
	w, h := size(box)
	if _, ok := p.src.(direct); ok && e.mask == nil && e.d != nil {
		if sw, sh := size(e.d.scene.laid); sw > 0 && sh > 0 && covers(box, e.d.scene.laid) {
			w, h = sw, sh
		}
	}
	p.src.Resize(int(math.Round(w)), int(math.Round(h)))
}

type direct interface{ DrawOn(dst *render.Image) }

func (p *picture) draw(e *Element, dst *render.Image) {
	box := shrink(e.box, e.padding)
	if _, ok := p.src.(glassed); ok {
		p.glass, p.drawn = box, true
	}
	if d, ok := p.src.(direct); ok && e.mask == nil && fills(box, dst) {
		d.DrawOn(dst)
		return
	}
	img := p.src.Draw()
	if img == nil {
		return
	}
	op := &render.DrawImageOptions{}
	w, h := size(box)
	bw, bh := math.Round(w), math.Round(h)
	if iw, ih := img.Bounds().Dx(), img.Bounds().Dy(); float64(iw) != bw || float64(ih) != bh {
		op.GeoM.Scale(bw/float64(iw), bh/float64(ih))
		op.Filter = render.FilterLinear
	}
	op.GeoM.Translate(math.Round(box.TopLeft.X), math.Round(box.TopLeft.Y))
	if e.mask != nil {
		dst.DrawImageIn(img, op, e.mask.points(box))
		return
	}
	dst.DrawImage(img, op)
}

func (*picture) needs(*Element) (w, h float64) { return 0, 0 }

func fills(box geom.AABB, dst *render.Image) bool {
	b := dst.Bounds()
	return covers(box, geom.NewAABB(geom.NewVec(float64(b.Min.X), float64(b.Min.Y)),
		geom.NewVec(float64(b.Max.X), float64(b.Max.Y))))
}

// covers reports whether box, rounded to pixels, covers the whole of screen
func covers(box, screen geom.AABB) bool {
	return math.Round(box.TopLeft.X) <= screen.TopLeft.X &&
		math.Round(box.TopLeft.Y) <= screen.TopLeft.Y &&
		math.Round(box.BottomRight.X) >= screen.BottomRight.X &&
		math.Round(box.BottomRight.Y) >= screen.BottomRight.Y
}

// Window is a panel with a title over its elements, one under another, each as large as it needs.
func Window(title string, elements ...*Element) *Element { return window(Label(title), elements) }

// WindowOf is a Window whose title t says, every frame; with no title it is left out, elements
// and all.
func WindowOf(title Text, elements ...*Element) *Element { return window(LabelOf(title), elements) }

func window(title *Element, elements []*Element) *Element {
	title.Padding(4)
	title.style = titleStyle
	parts := []Part{Fit(title)}
	for _, e := range elements {
		parts = append(parts, Fit(e))
	}
	w := newElement(&windowed{title: title}, Rows(parts...)).Padding(panelPadding)
	w.style, w.stroke = panelStyle, panelStroke
	return w
}

type windowed struct{ title *Element }

func (w *windowed) absent(*Element) bool { return w.title.absent() }

func (*windowed) place(e *Element, box geom.AABB) { e.children[0].lay(box) }

func (*windowed) draw(*Element, *render.Image) {}

func (*windowed) needs(e *Element) (w, h float64) { return e.children[0].needs() }

// Blank is an element showing nothing of its own: a gap, or with a Fill a divider, a plate.
func Blank() *Element { return newElement(blank{}) }

type blank struct{}

func (blank) place(*Element, geom.AABB)     {}
func (blank) draw(*Element, *render.Image)  {}
func (blank) needs(*Element) (w, h float64) { return 0, 0 }

// Drawer is an element drawn by r, a screen renderer drawing in the screen's pixels: the
// telemetry line. An r that is a plugin.Publisher is published and swapped with the scene.
func Drawer(r render.Renderer) *Element { return newElement(&layered{r: r}) }

type layered struct{ r render.Renderer }

func (*layered) place(*Element, geom.AABB)            {}
func (l *layered) draw(_ *Element, dst *render.Image) { l.r.Draw(dst) }
func (*layered) needs(*Element) (w, h float64)        { return 0, 0 }
func (*layered) container()                           {} // takes no click

// onGlass has the source draw on the glass over the picture, where it was drawn this frame
func (p *picture) onGlass(screen *render.Image) {
	if !p.drawn {
		return
	}
	p.drawn = false
	box := p.glass
	dst := screen.SubImage(image.Rect(int(math.Round(box.TopLeft.X)), int(math.Round(box.TopLeft.Y)),
		int(math.Round(box.BottomRight.X)), int(math.Round(box.BottomRight.Y))))
	p.src.(glassed).DrawGlass(dst)
}
