package ui

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
)

// Scene is a game.Scene whose screen is a tree of elements: Pictures are its pictures of the world,
// each initialised once however many feeds show it, and Screen the elements laid over the screen.
// Both are asked for once, as the Stage is entered.
type Scene struct {
	name     string
	pictures func() []render.WorldRenderer
	screen   func() *Element
	root     *Element
	input    func(*control.InputEvents, game.Runtime, game.Composition)
	keys     []control.Binding
	issue    func(cmd any) error
	passed   control.InputEvents // what of this tick's input goes on to input
}

var _ game.Scene = (*Scene)(nil)

// NewScene is the scene name: the pictures of the world it shows, and its screen.
func NewScene(name string, pictures func() []render.WorldRenderer, screen func() *Element) *Scene {
	return &Scene{name: name, pictures: pictures, screen: screen}
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
	var pictures []render.WorldRenderer
	if s.pictures != nil {
		pictures = s.pictures()
	}
	s.root = s.screen()
	return []render.Layer{&drawing{scene: s, pictures: pictures}}
}

func (s *Scene) Focusable() bool { return true }

// Show shows every element called name.
func (s *Scene) Show(name string) { s.each(name, func(e *Element) { e.hidden = false }) }

// Hide hides every element called name.
func (s *Scene) Hide(name string) { s.each(name, func(e *Element) { e.hidden = true }) }

// Toggle shows the elements called name where any is hidden, else hides them.
func (s *Scene) Toggle(name string) {
	hidden := false
	s.each(name, func(e *Element) { hidden = hidden || e.hidden })
	s.each(name, func(e *Element) { e.hidden = !hidden })
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
	pictures []render.WorldRenderer
	pins     []*Element // the elements pinned to entities

	query  *goke.Query // every entity carrying effect markers, and its place if it has one
	states goke.Comp[tag.Tags[effect.States]]
	base   goke.OptComp[entity.Base]
	z      goke.OptComp[entity.Z]
	label  goke.OptComp[entity.Label]
	spots  map[*Element][]spot
}

func (d *drawing) Init(si *goke.SysInit) {
	seen := map[render.WorldRenderer]bool{}
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
	root.lay(box)
	if len(d.pins) > 0 {
		d.find()
		vs := views(root)
		for _, e := range d.pins {
			e.pin.stand(e, d.spots[e], vs, box)
		}
	}
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
