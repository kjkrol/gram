package game

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
)

// Initializer is what a Stage gets during Init to install plugins and configure the ECS.
type Initializer interface {
	plugin.Installer
	Use(p plugin.Plugin) error

	// UseWorld builds and installs this Stage's world.Plugin from cfg; a second call panics.
	UseWorld(cfg world.Config) *world.Plugin

	// Track saves and loads s alongside the game's Plugins; tracked after a Load — a Stage's
	// scenes, made once the world is there — s gets the state the save holds for it at once.
	Track(s plugin.Serializable) error

	// TPS returns the engine's measured-ticks-per-second counter.
	TPS() *TPS
}
