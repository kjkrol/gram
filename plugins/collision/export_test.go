package collision

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
)

// New builds the collision engine over space, without a world plugin: for the tests.
func New(space *aabbworld.Space, ecs *goke.ECS) *module {
	return newModule(space, ecs, &host.PairHost[Meeting]{}, &host.EachHost[Struck]{})
}

// Hook hosts rules of Meeting, a pair, or of Struck on the engine New builds.
func (m *module) Hook(rules ...plugin.Rule) error {
	return hostAll(m.pairs, m.entities, rules)
}

// NewCollisionSystem builds the collision system over space, no rules hosted: for the tests.
func NewCollisionSystem(space *aabbworld.Space) *collisionSystem {
	return newCollisionSystem(space, &host.PairHost[Meeting]{}, &host.EachHost[Struck]{}, nil)
}
