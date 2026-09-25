package game

import (
	"image"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Scene groups the renderers and input handling for one thing a Stage can show.
type Scene interface {
	// Name uniquely identifies this scene.
	Name() string

	// Layers returns this scene's layers, bottom to top: render.Renderers drawn on the screen and
	// render.WorldRenderers drawn through each viewport; called once, when the Stage is entered.
	Layers() []render.Layer

	// HandleEvents handles this tick's input, while this scene is active.
	HandleEvents(events *control.InputEvents, runtime Runtime, composition Composition)

	// Focusable reports whether this scene can ever become active.
	Focusable() bool
}

// Viewer is a Scene showing the world: its WorldRenderers are drawn once per viewport it gives
// for the screen, every frame — a player's camera over the whole screen, two halves of a split
// screen, a minimap in a corner. A scene with world layers must be one.
type Viewer interface {
	Viewports(screen image.Rectangle) []render.Viewport
}
