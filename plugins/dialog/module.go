package dialog

import (
	"time"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/players/owner"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p        *Plugin
	sys      *talkSystem
	runnable goke.Runnable
	clock    *clock.Clock // nil: run at once
}

func newModule(p *Plugin) *module {
	return &module{p: p, sys: newTalkSystem(p), clock: p.w.Clock()}
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable != nil {
		return
	}
	m.runnable = ecs.RegSys(m.sys)
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.runnable, step)
		ctx.Sync()
	})
}

func (m *module) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(*goke.SysInit) {
		if m.p.talking == (effect.Effect{}) {
			panic("dialog: DefineEffects was not called where the Stage defines its effects")
		}
		m.p.check()
	}}}
}

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Script](), goke.LoadComp[Talk](), goke.LoadComp[tag.Tags[owner.Family]]()}
}
