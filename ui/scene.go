package ui

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/render"
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
}

func (d *drawing) Init(si *goke.SysInit) {
	seen := map[render.WorldRenderer]bool{}
	for _, p := range d.pictures {
		if !seen[p] {
			seen[p] = true
			p.Init(si)
		}
	}
}

func (d *drawing) Draw(screen *render.Image) {
	root := d.scene.root
	if root == nil {
		return
	}
	b := screen.Bounds()
	root.lay(geom.NewAABB(geom.NewVec(float64(b.Min.X), float64(b.Min.Y)), geom.NewVec(float64(b.Max.X), float64(b.Max.Y))))
	root.paint(screen)
}
