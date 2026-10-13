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

// Layers is the one layer drawing the screen.
func (s *Scene) Layers() []render.Layer {
	var pictures []render.Picture
	if s.root != nil {
		s.root.walk(func(e *Element) {
			if p, ok := e.content.(*picture); ok {
				if f, ok := p.src.(interface{ Picture() render.Picture }); ok && f.Picture() != nil {
					pictures = append(pictures, f.Picture())
				}
			}
		})
	}
	s.dress()
	if s.loaded {
		s.reveal()
	} else {
		s.note()
	}
	s.drawing = &drawing{scene: s, pictures: pictures}
	return []render.Layer{s.drawing}
}

func (s *Scene) Focusable() bool { return true }

// Lay lays the scene's elements over screen, as every frame does before drawing.
func (s *Scene) Lay(screen geom.AABB) {
	if s.root == nil {
		return
	}
	s.root.lay(screen)
	if d := s.drawing; d != nil && len(d.pins) > 0 {
		d.find()
		vs := views(s.root)
		for _, e := range d.pins {
			e.pin.stand(e, d.spots[e], vs, screen)
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
	scene    *Scene
	pictures []render.Picture
	pins     []*Element

	query  *goke.Query
	states goke.Comp[tag.Tags[effect.States]]
	base   goke.OptComp[entity.Base]
	z      goke.OptComp[entity.Z]
	label  goke.OptComp[ceremony.Label]
	spots  map[*Element][]spot
}

func (d *drawing) Init(si *goke.SysInit) {
	seen := map[render.Picture]bool{}
	for _, p := range d.pictures {
		if !seen[p] {
			seen[p] = true
			p.Init(si)
		}
	}
	if d.scene.root != nil {
		d.scene.root.walk(func(e *Element) {
			if e.pin != nil {
				d.pins = append(d.pins, e)
			}
			if l, ok := e.content.(*layered); ok {
				l.r.Init(si)
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
}

func (d *drawing) find() {
	for _, e := range d.pins {
		d.spots[e] = d.spots[e][:0]
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
				d.note(e, id, at(bases, i), at(zs, i), at(labels, i))
			}
		}
	}
	for _, e := range d.pins {
		if e.pin.where != nil {
			e.pin.where.each(func(id uid.UID64, base *entity.Base, z *entity.Z, label *ceremony.Label) {
				d.note(e, id, base, z, label)
			})
		}
	}
}

func (d *drawing) note(e *Element, id uid.UID64, base *entity.Base, z *entity.Z, label *ceremony.Label) {
	s := spot{id: id}
	if base != nil {
		s.placed = true
		s.x, s.y, s.z = e.pin.point(*base, z)
	} else if label == nil || *label == (ceremony.Label{}) {
		return // a cell: no place and no name
	}
	d.spots[e] = append(d.spots[e], s)
}
