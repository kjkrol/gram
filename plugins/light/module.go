package light

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p   *Plugin
	own goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) { m.own = ecs.RegSys(&ownSun{p: m.p}) }

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) { ctx.Run(m.own, d) }

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Source](), goke.LoadComp[Shining](),
		goke.LoadComp[Shadow]()}
}

// ownSun has the plugin's own entity shine the scene's distant light, and readies the plugin's
// queries
type ownSun struct {
	p       *Plugin
	q       *goke.Query
	shining goke.Comp[Shining]
}

func (s *ownSun) Init(si *goke.SysInit) {
	p := s.p
	s.q = si.NewQueryBuilder(&s.shining).Build()
	p.sources = si.NewQueryBuilder(&p.source).Optional(&p.shining).Optional(&p.base).
		Optional(&p.z).Build()
	p.blocks = si.NewQueryBuilder(&p.blockBase, &p.blockZ, &p.blockOf).Optional(&p.blockLie).
		Build()
}

func (s *ownSun) Update(*goke.CmdBuf, time.Duration) {
	if s.p.own != nil && s.q.Seek(s.p.Entity()) {
		s.shining.At(s.q.Cursor()).Light = *s.p.own
	}
	s.p.fresh = false
}
