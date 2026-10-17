package world

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/host"
)

// Hook hands rules to the world's own hosts, as the engine does with the roles' rules.
func (p *Plugin) Hook(rules ...act.Rule) error {
	return ceremony.Deliver([]host.Host{&p.module.moments.host}, rules...)
}
