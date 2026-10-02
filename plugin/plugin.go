package plugin

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Plugin extends a Game: Install wires an ECS module, setup, renderers and resources as one unit.
type Plugin interface {
	// Name uniquely identifies this plugin — Use rejects a duplicate.
	Name() string

	// Install queues this plugin's ECS wiring.
	Install(ctx Installer) error

	// RunPlan runs this plugin's per-tick work; call it from Stage.Update in the order you need.
	RunPlan(ctx goke.RunCtx, d time.Duration)

	// WithRenderer has this plugin's renderer draw sprites from atlas; call before Use.
	WithRenderer(atlas render.AtlasSource)

	// Renderer returns this plugin's own layer — a render.Renderer, or a render.Source a scene hands
	// to its render.Composer — or nil if it has none.
	Renderer() render.Layer

	// EventHandler returns this plugin's own control.EventHandler, or nil if it has none.
	EventHandler() control.EventHandler

	// Serializable returns this plugin's persistable state, or nil if it has none.
	Serializable() Serializable
}
