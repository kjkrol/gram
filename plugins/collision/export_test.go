package collision

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule"
)

// New builds the collision engine over space, without a world plugin: for the tests.
func New(space *aabbworld.Space, ecs *goke.ECS) *module {
	return newModule(space, ecs, &rule.PairHost[Meeting]{}, &rule.EachHost[Struck]{})
}

// Hook hosts rules of Meeting, a pair, or of Struck on the engine New builds.
func (m *module) Hook(rules ...rule.Rule) error {
	return hostAll(m.pairs, m.entities, rules)
}

// Carry has the commands the rules give go to commands, as the world's carrier takes them; call
// before RegSystems.
func (m *module) Carry(commands *control.Carrier) {
	m.tick = func(cb *goke.CmdBuf, dt time.Duration) rule.Tick {
		return rule.Tick{CmdBuf: cb, Now: time.Now(), Dt: dt, Commands: commands}
	}
}

// NewCollisionSystem builds the collision system over space, no rules hosted: for the tests.
func NewCollisionSystem(space *aabbworld.Space) *collisionSystem {
	return newCollisionSystem(space, &rule.PairHost[Meeting]{}, &rule.EachHost[Struck]{}, nil)
}
