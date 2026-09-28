package effects

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/uid"
)

// Idle marks an entity whose last effect has just ended; it stays for one tick, so anything
// watching for it sees it whatever the order of the plugins' passes.
type Idle struct{}

// Each is a behavior run once on every entity carrying T whose last effect has just ended.
// Register it with the world's RegisterBehavior.
func Each[T any](react func(t plugin.Tick, state *T, i Idling)) plugin.Behavior {
	return host.Each(react)
}

// Every is Each without a state component: every entity whose last effect has just ended.
func Every(react func(t plugin.Tick, i Idling)) plugin.Behavior { return host.Every(react) }

// Idling is what an Each behavior hosted by effects gets, once, for an entity carrying Idle.
type Idling struct{ ID uid.UID64 }
