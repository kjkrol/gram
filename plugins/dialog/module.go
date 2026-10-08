package dialog

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/rule/effect"
)

var _ goke.Module = (*module)(nil)

// module registers the talk system as dialog's per-tick system.
type module struct {
	p        *Plugin
	sys      *talkSystem
	runnable goke.Runnable
	clock    *clock.Clock // the world's; nil, run at once
}

func newModule(p *Plugin) *module {
	return &module{p: p, sys: newTalkSystem(p), clock: p.w.Clock()}
}

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems registers the talk system — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable != nil {
		return
	}
	m.runnable = ecs.RegSys(m.sys)
}

// RunPlan hands the conversations to the simulation: the tactical pause holds them too.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.runnable, step)
		ctx.Sync()
	})
}

// SetupSystems checks, once the Stage defined everything, that the effects were defined and that
// every answer leads to a node.
func (m *module) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(*goke.SysInit) {
		if m.p.talking == (effect.Effect{}) {
			panic("dialog: DefineEffects was not called where the Stage defines its effects")
		}
		m.p.check()
	}}}
}

// LoadComps lists the component types dialog owns, and the owners' tags it reads to know who may
// answer — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Script](), goke.LoadComp[Talk](), goke.LoadComp[Memory](),
		goke.LoadComp[tag.Tags[owner.Family]]()}
}
