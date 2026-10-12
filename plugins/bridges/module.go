package bridges

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

var _ goke.Module = (*module)(nil)

// module runs the plugin's systems: the hands once a tick, the bridges built every step of the
// simulation.
type module struct {
	p           *Plugin
	hands, done goke.Runnable
	built       bool
}

func newModule(p *Plugin) *module { return &module{p: p} }

// RegSystems builds and registers the plugin's systems — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	if m.built {
		return
	}
	m.hands = ecs.RegSys(handSystem{m.p})
	m.done = ecs.RegSys(buildSystem{m.p})
	m.built = true
}

// RunPlan carries out the players' commands at once, and builds what they confirmed every step.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.hands, d)
	clock.Simulate(m.p.worldPlugin.Clock(), ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.done, step)
	})
}

// SetupSystems is empty — the plugin seeds nothing of its own.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps is empty: the parts' components are their kinds', registered by the world.
func (m *module) LoadComps() []goke.CompToken { return nil }

// handSystem carries out the players' commands about the bridges in their hands.
type handSystem struct{ p *Plugin }

func (handSystem) Init(*goke.SysInit) {}

func (s handSystem) Update(*goke.CmdBuf, time.Duration) { s.p.carryOut() }

// buildSystem builds the bridges the players said to build.
type buildSystem struct{ p *Plugin }

func (buildSystem) Init(*goke.SysInit) {}

func (s buildSystem) Update(*goke.CmdBuf, time.Duration) {
	for _, b := range s.p.pending {
		s.p.build(b)
	}
	s.p.pending = s.p.pending[:0]
}
