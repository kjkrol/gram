package game

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Scene groups the renderers and input handling for one thing a Stage can show.
type Scene interface {
	// Name uniquely identifies this scene.
	Name() string

	// Drawers returns what draws this scene, bottom to top: render.Renderers drawn on the screen,
	// the world among them through a ui.Image of a render.Feed. It is called once, as the Stage is
	// entered.
	Drawers() []render.Renderer

	// HandleEvents handles this tick's input, while this scene is active.
	HandleEvents(events *control.InputEvents, runtime Runtime, composition Composition)

	// Focusable reports whether this scene can ever become active.
	Focusable() bool
}

// Scenic is a plugin with scenes of its own, which a Stage defined in sections keeps in its stack
// hidden until something shows them.
type Scenic interface {
	Scenes() []Scene
}
