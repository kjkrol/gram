package light

import "github.com/kjkrol/gram/plugins/world"

// Lighting is what the world asks of the light as it publishes its entities.
func Lighting(p *Plugin) world.Lighting { return lighting{p} }

// Gathers is how many times the plugin gathered the Baked blocks anew.
func Gathers(p *Plugin) int { return p.gathers }
