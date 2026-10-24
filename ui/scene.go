package ui

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Scene is a game.Scene whose screen is a tree of elements laid over the screen every frame.
type Scene struct {
	name    string
	root    *Element
	input   func(*control.InputEvents, game.Runtime, game.Composition)
	keys    []control.Binding
	issue   func(cmd command.Command) error
	runtime game.Runtime
	passed  control.InputEvents
	shown   []string
	loaded  bool
	drawing *drawing
	theme   Theme
	overlay *Element  // a dropdown's options, over everything, hanging from field
	field   geom.AABB // the dropdown's box
	held    *Element  // a slider dragged
	laid    geom.AABB // the screen the scene was last laid over
}

var _ game.Scene = (*Scene)(nil)
var _ plugin.Serializable = (*Scene)(nil)
var _ plugin.Restorer = (*Scene)(nil)

// NewScene is the scene name, its screen the element screen, made once the world is there.
func NewScene(name string, screen *Element) *Scene {
	return &Scene{name: name, root: screen, theme: DefaultTheme()}
}

// Input hands the scene's input, while it is active, to fn: the players' bindings, as a rule.
func (s *Scene) Input(fn func(*control.InputEvents, game.Runtime, game.Composition)) *Scene {
	s.input = fn
	return s
}

func (s *Scene) Name() string { return s.name }

// Drawers is the one drawer drawing the screen, which publishes what it reads off the game once a
// frame (plugin.Publisher).
func (s *Scene) Drawers() []render.Renderer {
	s.dress()
	if s.loaded {
		s.reveal()
	} else {
		s.note()
	}
	s.drawing = &drawing{scene: s}
	s.bind()
	return []render.Renderer{s.drawing}
}

// bind has the elements read what the scene's drawing publishes
func (s *Scene) bind() {
	if s.root == nil || s.drawing == nil {
		return
	}
	s.root.walk(func(e *Element) {
		e.d = s.drawing
		if e.pin != nil && e.pin.show != nil {
			e.pin.show.d = s.drawing
		}
	})
}

func (s *Scene) Focusable() bool { return true }

// Replace puts screen in place of the scene's tree, for one built afresh each time it is shown
// (a listing): dressed in the theme at once, drawn from the next frame. What Drawers binds once —
// pictures, screen renderers, pinned elements, texts read off components — must not be in it.
func (s *Scene) Replace(screen *Element) {
	s.root = screen
	s.overlay, s.held = nil, nil
	s.dress()
	s.note()
	s.bind()
}

// Unfolded reports whether a dropdown's options lie open: Esc closes them before anything else.
func (s *Scene) Unfolded() bool { return s.overlay != nil }

// Laid is the screen the scene was last laid over: a zero box before the first frame.
func (s *Scene) Laid() geom.AABB { return s.laid }

// Lay lays the scene's elements over screen, as every frame does before drawing.
func (s *Scene) Lay(screen geom.AABB) {
	s.laid = screen
	if s.root == nil {
		return
	}
	s.root.lay(screen)
	if ov := s.overlay; ov != nil {
		w, h := ov.needs()
		fw, _ := size(s.field)
		at := geom.NewVec(s.field.TopLeft.X, s.field.BottomRight.Y)
		if at.Y+h > screen.BottomRight.Y {
			at.Y = s.field.TopLeft.Y - h
		}
		ov.lay(geom.NewAABB(at, at.Add(geom.NewVec(max(w, fw), h))))
	}
	if d := s.drawing; d != nil && len(d.pins) > 0 {
		spots := d.spots
		if f := d.front(); f != nil {
			spots = f.spots
		} else {
			d.find(spots)
		}
		vs := views(s.root)
		for _, e := range d.pins {
			e.pin.stand(e, spots[e], vs, screen)
			// laid where it stands, so its Box is by its entity
			e.pin.each(e, e.parent, func(*instance) {})
		}
	}
}

// Show shows every element called name.
func (s *Scene) Show(name string) {
	s.each(name, func(e *Element) { e.hidden = false })
	s.note()
}

// Hide hides every element called name.
func (s *Scene) Hide(name string) {
	s.each(name, func(e *Element) { e.hidden = true })
	s.note()
}

// Toggle shows the elements called name where any is hidden, else hides them.
func (s *Scene) Toggle(name string) {
	hidden := false
	s.each(name, func(e *Element) { hidden = hidden || e.hidden })
	s.each(name, func(e *Element) { e.hidden = !hidden })
	s.note()
}

// Element is the first element called name, laid where the last frame laid it; nil for none.
func (s *Scene) Element(name string) *Element {
	var found *Element
	s.each(name, func(e *Element) {
		if found == nil {
			found = e
		}
	})
	return found
}

// Shown reports whether an element called name is shown.
func (s *Scene) Shown(name string) bool {
	shown := false
	s.each(name, func(e *Element) { shown = shown || !e.hidden })
	return shown
}

// Persisted is the names of the elements shown: a game saved with a window open loads with it open.
func (s *Scene) Persisted() []any { return []any{&s.shown} }

// Restore lays the names loaded on the elements, now or once they are made.
func (s *Scene) Restore() {
	s.loaded = true
	s.reveal()
}

func (s *Scene) note() {
	s.shown = s.shown[:0]
	if s.root == nil {
		return
	}
	s.root.walk(func(e *Element) {
		if e.name != "" && !e.hidden && !slices.Contains(s.shown, e.name) {
			s.shown = append(s.shown, e.name)
		}
	})
}

func (s *Scene) reveal() {
	if s.root == nil {
		return
	}
	s.root.walk(func(e *Element) {
		if e.name != "" {
			e.hidden = !slices.Contains(s.shown, e.name)
		}
	})
}

func (s *Scene) each(name string, fn func(*Element)) {
	if s.root == nil {
		return
	}
	s.root.walk(func(e *Element) {
		if e.name == name {
			fn(e)
		}
	})
}

type drawing struct {
	scene *Scene
	pins  []*Element

	query  *goke.Query
	states goke.Comp[tag.Tags[effect.States]]
	base   goke.OptComp[entity.Base]
	z      goke.OptComp[entity.Z]
	label  goke.OptComp[ceremony.Label]
	spots  map[*Element][]spot // found as the scene is laid, before any is published

	published render.Buffered[published]
	swapped   bool
	// the cameras GoToIt moves onto an entity: asked as the scene is laid, handed over at Swap,
	// moved at the next Publish
	wished, looks []look
}

// published is what a scene draws of the game, as a Publish read it: where its pinned elements'
// entities stand and whether each picture shows them, each label's words for the entity it is
// drawn for, each checkbox's tick, each slider's number and which elements are hidden
type published struct {
	spots  map[*Element][]spot
	shows  map[showing]bool
	words  map[saying]said
	ticks  map[*checkbox]bool
	values map[*slider]float64
	hidden map[*Element]bool
}

type showing struct {
	pic *Element
	id  uid.UID64
}

type saying struct {
	l      *label
	of     uid.UID64
	pinned bool
}

type said struct {
	text string
	ok   bool
}

type look struct {
	looker Looker
	id     uid.UID64
}

func (p *published) reset() {
	if p.spots == nil {
		*p = published{spots: map[*Element][]spot{}, shows: map[showing]bool{},
			words: map[saying]said{}, ticks: map[*checkbox]bool{}, values: map[*slider]float64{},
			hidden: map[*Element]bool{}}
		return
	}
	for e, s := range p.spots {
		p.spots[e] = s[:0]
	}
	clear(p.shows)
	clear(p.words)
	clear(p.ticks)
	clear(p.values)
	clear(p.hidden)
}

// Publish reads what the scene draws off the game, for the frame drawn next: the entities of its
// pinned elements, its words, its controls and what is hidden; and moves the cameras its pinned
// elements asked to.
func (d *drawing) Publish() {
	for _, l := range d.looks {
		l.looker.LookAt(l.id)
	}
	d.looks = d.looks[:0]
	b := d.published.Back()
	b.reset()
	root := d.scene.root
	if root == nil {
		return
	}
	if len(d.pins) > 0 {
		d.find(b.spots)
		d.showing(b, root)
	}
	d.read(b, root, nil)
	root.walk(func(e *Element) {
		if p, ok := drawerOf(e).(plugin.Publisher); ok {
			p.Publish()
		}
	})
}

// Swap has what was published last drawn, and hands the cameras asked to move to Publish.
func (d *drawing) Swap() {
	d.published.Swap()
	d.swapped = true
	d.looks = append(d.looks, d.wished...)
	d.wished = d.wished[:0]
	if root := d.scene.root; root != nil {
		root.walk(func(e *Element) {
			if p, ok := drawerOf(e).(plugin.Publisher); ok {
				p.Swap()
			}
		})
	}
}

// front is what was published last, nil before any was
func (d *drawing) front() *published {
	if d == nil || !d.swapped {
		return nil
	}
	return d.published.Front()
}

// drawerOf is the renderer an element draws, nil for any other
func drawerOf(e *Element) render.Renderer {
	if l, ok := e.content.(*layered); ok {
		return l.r
	}
	return nil
}

// read reads off the game what e and its children draw: of a pinned one's, for each of spots
func (d *drawing) read(b *published, e *Element, spots []spot) {
	b.hidden[e] = e.hidden
	if e.hidden {
		return
	}
	if e.pin != nil {
		spots = b.spots[e]
	}
	switch c := e.content.(type) {
	case *label:
		if c.src != nil {
			b.say(c, 0, false)
			for _, s := range spots {
				b.say(c, s.id, true)
			}
		}
	case *checkbox:
		b.ticks[c] = c.get()
	case *slider:
		b.values[c] = c.get()
	}
	for _, c := range e.children {
		d.read(b, c, spots)
	}
}

func (b *published) say(l *label, of uid.UID64, pinned bool) {
	text, ok := l.src.Text(of, pinned)
	b.words[saying{l, of, pinned}] = said{text, ok}
}

// showing notes whether each picture's Owner shows the pinned elements' entities
func (d *drawing) showing(b *published, root *Element) {
	root.walk(func(e *Element) {
		p, ok := e.content.(*picture)
		if !ok {
			return
		}
		o, ok := p.input.(Owner)
		if !ok {
			return
		}
		for _, pinned := range d.pins {
			for _, s := range b.spots[pinned] {
				b.shows[showing{e, s.id}] = o.Shows(s.id)
			}
		}
	})
}

func (d *drawing) Init(si *goke.SysInit) {
	if d.scene.root != nil {
		d.scene.root.walk(func(e *Element) {
			if e.pin != nil {
				d.pins = append(d.pins, e)
			}
			if l, ok := e.content.(*layered); ok {
				if b, ok := l.r.(plugin.Bound); ok {
					b.Init(si)
				}
			}
			if l, ok := e.content.(*label); ok {
				if b, ok := l.src.(bound); ok {
					b.bind(si)
				}
			}
		})
	}
	if len(d.pins) > 0 {
		d.query = si.NewQueryBuilder(&d.states).Optional(&d.base).Optional(&d.z).Optional(&d.label).Build()
		d.spots = map[*Element][]spot{}
	}
	for _, e := range d.pins {
		if e.pin.where != nil {
			e.pin.where.bind(si)
		}
	}
}

func (d *drawing) Draw(screen *render.Image) {
	root := d.scene.root
	if root == nil {
		return
	}
	b := screen.Bounds()
	box := geom.NewAABB(geom.NewVec(float64(b.Min.X), float64(b.Min.Y)), geom.NewVec(float64(b.Max.X), float64(b.Max.Y)))
	d.scene.Lay(box)
	root.paint(screen)
	if ov := d.scene.overlay; ov != nil {
		ov.paint(screen)
	}
	root.walk(func(e *Element) {
		if p, ok := e.content.(*picture); ok {
			p.onGlass(screen)
		}
	})
}

// find finds the entities of the pinned elements, into spots
func (d *drawing) find(spots map[*Element][]spot) {
	for _, e := range d.pins {
		spots[e] = spots[e][:0]
	}
	for d.query.All(); d.query.Next(); {
		cur := d.query.Cursor()
		states, bases, zs, labels := d.states.Slice(cur), d.base.Slice(cur), d.z.Slice(cur), d.label.Slice(cur)
		for i, id := range cur.IDs {
			for _, e := range d.pins {
				p := e.pin
				switch {
				case p.under != nil && states[i].Has(p.under.Mark()):
				case labels != nil && !p.on.Nobody() && p.on.Holds(labels[i]):
				default:
					continue
				}
				note(spots, e, id, at(bases, i), at(zs, i), at(labels, i))
			}
		}
	}
	for _, e := range d.pins {
		if e.pin.where != nil {
			e.pin.where.each(func(id uid.UID64, base *entity.Base, z *entity.Z, label *ceremony.Label) {
				note(spots, e, id, base, z, label)
			})
		}
	}
}

func note(spots map[*Element][]spot, e *Element, id uid.UID64, base *entity.Base, z *entity.Z,
	label *ceremony.Label) {
	s := spot{id: id}
	if base != nil {
		s.placed = true
		s.x, s.y, s.z = e.pin.point(*base, z)
	} else if label == nil || *label == (ceremony.Label{}) {
		return // a cell: no place and no name
	}
	spots[e] = append(spots[e], s)
}
