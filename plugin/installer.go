package plugin

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/render"
)

// Installer is what a Plugin gets during Install: ECS wiring, Hosts, which tells the engine the
// hosts of the rules of the moments the plugin catches, and what the plugin draws.
type Installer interface {
	UseModule(m goke.Module)
	Setup(providers ...goke.SetupProvider)
	RegSys(factory func() goke.System) goke.Runnable
	ECS() *goke.ECS
	Hosts(hosts ...host.Host)

	// Draws adds what the plugin draws to its Stage's picture, ordered there by height.
	Draws(sources ...render.Source)

	// Looks has declare run once the Stage is set up, before its scenes are made: the plugin's
	// looks declared and its atlases closed.
	Looks(declare func())

	// Pool is the goroutines the game shares the work a plugin splits among; nil, one.
	Pool() *parallel.Pool
}
