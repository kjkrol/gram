package plugin

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

// Plugin is the one extension point: what a Stage uses, named for the saves.
type Plugin interface {
	// Name is unique among a Stage's plugins; Use refuses a duplicate.
	Name() string

	// Install queues the plugin's ECS wiring.
	Install(ctx Installer) error

	// RunPlan is the plugin's work of a tick, called from Stage.Update in the order the game needs.
	RunPlan(ctx goke.RunCtx, d time.Duration)

	// Renderer is the plugin's own layer, or nil for none.
	Renderer() render.Layer

	// EventHandler is the plugin's own, or nil for none.
	EventHandler() control.EventHandler

	// Serializable is the plugin's saved state, or nil for none.
	Serializable() Serializable
}
