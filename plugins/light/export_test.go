package light

import "github.com/kjkrol/gram/plugins/world"

// Lighting is what the world asks of the light as it publishes its entities.
func Lighting(p *Plugin) world.Lighting { return lighting{p} }
