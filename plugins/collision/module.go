package collision

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
)

var _ goke.Module = (*module)(nil)

// module is the optional collision engine, run over a *aabbworld.Space borrowed from world.
type module struct {
	space *aabbworld.Space
	ecs   *goke.ECS

	pairs    *plugin.PairRules[Meeting]
	entities *plugin.Rules[Struck]

	system  goke.Runnable
	fieldOf func() Field
	heights bool // the world has heights: the system minds every Band
	stats   *ContactStats
	log     *log.Logger
	clock   *clock.Clock      // the world's; nil, run at once
	tick    plugin.TickSource // the world's, for the rules
	built   bool
}

func newModule(space *aabbworld.Space, ecs *goke.ECS, pairs *plugin.PairRules[Meeting], entities *plugin.Rules[Struck]) *module {
	return &module{space: space, ecs: ecs, pairs: pairs, entities: entities}
}

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems builds and registers the collision systems — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	if !m.built {
		m.build()
	}
}

// RunPlan hands the collisions to the simulation: every step, the pairs meet and are pushed apart.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.system, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — the collision engine has no one-time seeding of its own.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types the collision engine owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Collider](),
		goke.LoadComp[Physics](),
		goke.LoadComp[Sweep](),
	}
}

// =================================================================
// collision-specific
// =================================================================

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

func (m *module) build() {
	s := newCollisionSystem(m.space, m.pairs, m.entities, m.fieldOf)
	s.stats, s.log, s.heights = m.stats, m.log, m.heights
	s.tickOf = m.tick
	m.system = m.ecs.RegSys(s)
	m.built = true
}
