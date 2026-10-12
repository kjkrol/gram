package players

import (
	"fmt"
	"strings"
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p     *Plugin
	gives goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.gives = ecs.RegSys(&giveSystem{gives: &m.p.gives})
}

// RunPlan hands over the units given, then issues the KeyHeld commands of the keys still down
// for the next tick; call it after the plugins that drain theirs.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.gives, d)
	ctx.Sync()
	eventHandler{m.p}.hold()
}

// SetupSystems refuses a binding whose command no handler takes.
func (m *module) SetupSystems() []goke.System {
	return []goke.System{owning{m.p}, goke.SystemFn{OnInit: func(*goke.SysInit) {
		var missing []string
		for _, pl := range m.p.players {
			for _, b := range pl.bindings {
				if !m.p.worldPlugin.Carrier().Takes(b.Type()) {
					missing = append(missing, fmt.Sprintf("%v (%q for %s)", b.Type(), b.Label, pl.Name))
				}
			}
		}
		if len(missing) > 0 {
			panic("players: no plugin listens for " + strings.Join(missing, ", "))
		}
	}}}
}

// LoadComps is empty: players own no components.
func (m *module) LoadComps() []goke.CompToken { return nil }
