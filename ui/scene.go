package ui

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
)

// Scene is a game.Scene whose screen is a tree of elements: Pictures are its pictures of the world,
// each initialised once however many feeds show it, and Screen the elements laid over the screen.
// Both are asked for once, as the Stage is entered.
type Scene struct {
	name     string
	pictures func() []render.Picture
	screen   func() *Element
	root     *Element
	input    func(*control.InputEvents, game.Runtime, game.Composition)
	keys     []control.Binding
	issue    func(cmd any) error
	passed   control.InputEvents // what of this tick's input goes on to input
	shown    []string            // the names of the elements shown, as saved
	loaded   bool                // shown came from a save: laid on the elements once they are made
	drawing  *drawing            // the scene's layer, which finds the entities its pinned elements are for
	theme    Theme
}

var _ game.Scene = (*Scene)(nil)
var _ plugin.Serializable = (*Scene)(nil)
var _ plugin.Restorer = (*Scene)(nil)

// NewScene is the scene name: the pictures of the world it shows, and its screen.
func NewScene(name string, pictures func() []render.Picture, screen func() *Element) *Scene {
	return &Scene{name: name, pictures: pictures, screen: screen, theme: DefaultTheme()}
}

// Input hands the scene's input, while it is active, to fn: the players' bindings, as a rule.
func (s *Scene) Input(fn func(*control.InputEvents, game.Runtime, game.Composition)) *Scene {
	s.input = fn
	return s
}

func (s *Scene) Name() string { return s.name }

// Layers is the one layer drawing the screen: it initialises the pictures, lays the elements over
// the screen every frame and draws them.
func (s *Scene) Layers() []render.Layer {
	var pictures []render.Picture
	if s.pictures != nil {
		pictures = s.pictures()
	}
	s.root = s.screen()
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

// Lay lays the scene's elements over screen, as every frame does before drawing: the pictures
// get their sizes, the players where their pictures lie, the pinned elements their entities.
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
			e.pin.each(e, e.parent, func(*instance) {}) // laid where it stands: its Box is by its entity
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

// note writes down the names of the elements shown.
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

// reveal shows the named elements shown and hides the rest.
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

// drawing is the scene's layer: the screen's elements, laid and drawn every frame.
type drawing struct {
	scene    *Scene
	pictures []render.Picture
	pins     []*Element // the elements pinned to entities

	query  *goke.Query // every entity carrying effect markers, and its place if it has one
	states goke.Comp[tag.Tags[effect.States]]
	base   goke.OptComp[entity.Base]
	z      goke.OptComp[entity.Z]
	label  goke.OptComp[entity.Label]
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
		})
	}
	if len(d.pins) > 0 {
		d.query = si.NewQueryBuilder(&d.states).Optional(&d.base).Optional(&d.z).Optional(&d.label).Build()
		d.spots = map[*Element][]spot{}
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

// find notes, for every pinned element, the entities it is shown for and their points: those with
// a place in the world, and those with none that are called something (the world's own entity, a
// plugin's).
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
				s := spot{id: id}
				if bases != nil {
					var z *entity.Z
					if zs != nil {
						z = &zs[i]
					}
					s.placed = true
					s.x, s.y, s.z = p.point(bases[i], z)
				} else if labels == nil || labels[i] == (entity.Label{}) {
					continue // a cell, say: no place known here, and nothing that calls it
				}
				d.spots[e] = append(d.spots[e], s)
			}
		}
	}
}
