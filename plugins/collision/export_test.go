package collision

import (
	"errors"
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
)

// New builds the collision engine over space, without a world plugin: for the tests.
func New(space *aabbworld.Space, ecs *goke.ECS) *module {
	return newModule(space, ecs, &plugin.PairRules[Meeting]{}, &plugin.Rules[Struck]{})
}

// Hook hosts rules of Meeting, a pair, or of Struck on the engine New builds.
func (m *module) Hook(rules ...rule.Rule) error {
	return hostAll(m.pairs, m.entities, rules)
}

// Carry has the commands the rules give go to commands, as the world's carrier takes them; call
// before RegSystems.
func (m *module) Carry(commands *control.Carrier) {
	m.tick = func(cb *goke.CmdBuf, dt time.Duration) plugin.Tick {
		return plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: dt, Commands: commands}
	}
}

// NewCollisionSystem builds the collision system over space, no rules hosted: for the tests.
func NewCollisionSystem(space *aabbworld.Space) *collisionSystem {
	return newCollisionSystem(space, &plugin.PairRules[Meeting]{}, &plugin.Rules[Struck]{}, nil)
}

// Hook hands rules to the plugin's own hosts, as the engine does with the roles' rules through
// plugin.Installer.Hosts: the tests' way in.
func (p *Plugin) Hook(rules ...rule.Rule) error {
	return hostAll(&p.pairs, &p.entities, rules)
}

// hostAll hands each rule to whichever host takes it, stopping at the first neither does.
func hostAll(pairs *plugin.PairRules[Meeting], entities *plugin.Rules[Struck], rules []rule.Rule) error {
	for _, b := range rules {
		err := pairs.Add(b)
		if errors.Is(err, plugin.ErrUnhosted) {
			err = entities.Add(b)
		}
		if errors.Is(err, plugin.ErrUnhosted) {
			return fmt.Errorf("%w in collision — it takes a rule of Meeting or of Struck", err)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
