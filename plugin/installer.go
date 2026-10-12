package plugin

import "github.com/kjkrol/goke/v3"

// Installer is what a Plugin gets during Install: ECS wiring, and Hosts, which tells the engine
// the hosts of the rules of the moments the plugin catches.
type Installer interface {
	UseModule(m goke.Module)
	Setup(providers ...goke.SetupProvider)
	RegSys(factory func() goke.System) goke.Runnable
	ECS() *goke.ECS
	Hosts(hosts ...Host)
}

// Host takes the rules of the moments one pass of a plugin catches: a Rules, a PairRules or a
// StepRules. Add refuses a rule of another moment with ErrUnhosted, any rule once its system is
// built with ErrHostBuilt.
type Host interface {
	Add(rule any) error
}
