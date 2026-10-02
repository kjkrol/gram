package game

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// Initializer is what a Stage gets during Init to install plugins and configure the ECS.
type Initializer interface {
	plugin.Installer
	Use(p plugin.Plugin) error

	// UseWorld builds and installs this Stage's world.Plugin from cfg; a second call panics.
	UseWorld(cfg world.Config) *world.Plugin

	// Hook hooks each rule on the plugin this Stage uses that hosts its moment — a rule of a
	// unit.Standing on the board, of a vision.Sighting on vision — so a Stage need not know which
	// (rule.HookOn); one no plugin in use hosts is plugin.ErrUnhosted. Call it once the plugins are
	// Used, before Init returns.
	Hook(rules ...rule.Rule) error

	// Track saves and loads s alongside the game's Plugins.
	Track(s plugin.Serializable) error

	// TPS returns the engine's measured-ticks-per-second counter.
	TPS() *TPS
}
