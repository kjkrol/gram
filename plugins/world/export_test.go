package world

import (
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/act"
)

// Hook hands rules to the world's own hosts, as the engine does with the roles' rules.
func (p *Plugin) Hook(rules ...act.Rule) error {
	return hosts.Deliver([]plugin.Host{p.module.movers, p.module.leavers, &p.module.moments.host}, rules...)
}
