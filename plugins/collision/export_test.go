package collision

import (
	"errors"
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
)

// New builds the collision module over space without a world plugin.
func New(space *aabbworld.Space, ecs *goke.ECS) *module {
	return newModule(space, ecs, &host.PairRules[Meeting]{}, &host.Rules[Struck]{})
}

// Hook hosts rules of Meeting or of Struck on the module.
func (m *module) Hook(rules ...act.Rule) error {
	return hostAll(m.pairs, m.entities, rules)
}

// Carry has the commands the rules give go to commands; call before RegSystems.
func (m *module) Carry(commands *command.Carrier) {
	m.tick = func(dt time.Duration) host.Tick {
		return host.Tick{Now: time.Now(), Dt: dt, Commands: commands}
	}
}

// NewCollisionSystem builds the collision system over space, no rules hosted.
func NewCollisionSystem(space *aabbworld.Space) *collisionSystem {
	return newCollisionSystem(space, &host.PairRules[Meeting]{}, &host.Rules[Struck]{}, nil)
}

// Hook hands rules to the plugin's own hosts, as the engine does through plugin.Installer.Hosts.
func (p *Plugin) Hook(rules ...act.Rule) error {
	return hostAll(&p.pairs, &p.entities, rules)
}

func hostAll(pairs *host.PairRules[Meeting], entities *host.Rules[Struck], rules []act.Rule) error {
	for _, b := range rules {
		err := pairs.Add(b)
		if errors.Is(err, host.ErrUnhosted) {
			err = entities.Add(b)
		}
		if errors.Is(err, host.ErrUnhosted) {
			return fmt.Errorf("%w in collision — it takes a rule of Meeting or of Struck", err)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
