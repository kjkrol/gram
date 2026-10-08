package plugin

import "github.com/kjkrol/goke/v3"

// Installer is what a Plugin gets during Install — ECS wiring only.
// Cross-plugin data/behavior comes from constructor injection instead.
type Installer interface {
	UseModule(m goke.Module)
	Setup(providers ...goke.SetupProvider)
	RegSys(factory func() goke.System) goke.Runnable
	ECS() *goke.ECS
	// Hosts tells the engine the hosts of the rules of the moments the plugin catches: the roles'
	// rules are handed to them once the Stage's Init returns.
	Hosts(hosts ...Host)
}

// Host takes the rules of the moments one pass of a plugin catches — a Rules, a PairRules, a
// StepRules: Add refuses a rule of another moment with ErrUnhosted, any rule once its system is
// built with ErrHostBuilt.
type Host interface {
	Add(rule any) error
}
