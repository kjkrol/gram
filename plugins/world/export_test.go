package world

import (
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
)

// Hook hands rules to the world's own hosts, as the engine does with the roles' rules through
// plugin.Installer.Hosts: the tests' way in.
func (p *Plugin) Hook(rules ...rule.Rule) error {
	return hosts.Deliver([]plugin.Host{p.module.movers, p.module.leavers, &p.module.moments.host}, rules...)
}
