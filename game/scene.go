package game

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Scene groups the renderers and input handling for one thing a Stage can show.
type Scene interface {
	// Name uniquely identifies this scene.
	Name() string

	// Layers returns this scene's layers, bottom to top, render.Renderers drawn on the screen — the
	// world among them through a ui.Image of a render.Feed (ui.Scene); called once, when the Stage
	// is entered.
	Layers() []render.Layer

	// HandleEvents handles this tick's input, while this scene is active.
	HandleEvents(events *control.InputEvents, runtime Runtime, composition Composition)

	// Focusable reports whether this scene can ever become active.
	Focusable() bool
}

// Scenic is a plugin with scenes of its own — the players' list of shortcuts: a Stage defined in
// sections has them in its stack after the game's own, hidden until something shows them.
type Scenic interface {
	Scenes() []Scene
}
