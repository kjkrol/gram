package selection

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

type module struct {
	sys      *SelectionSystem
	runnable goke.Runnable
	ready    func() // the reads' queries, once the system's are built
}

var _ goke.Module = (*module)(nil)

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable != nil {
		return
	}
	m.runnable = ecs.RegSys(m.sys)
	ecs.RegSys(goke.SystemFn{OnInit: func(*goke.SysInit) { m.ready() }})
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return nil // the family's tags are the world's Kinds'
}
