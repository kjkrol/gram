package players

import (
	"fmt"
	"strings"
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

// module runs the Gives and, once every plugin is installed, checks that each bound command has
// an owner.
type module struct {
	p     *Plugin
	gives goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.gives = ecs.RegSys(&giveSystem{gives: &m.p.gives})
}

// RunPlan hands over the units given, then issues the KeyHeld commands of the keys still down, for the next tick; call it
// after the plugins that drain theirs. A command waits in its queue for its handler's pass:
// nothing is dropped, whoever gave it — a player, or an entity after that pass.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.gives, d)
	ctx.Sync()
	m.p.hands.stale = true // the next reader sums the Drives given from here on
	eventHandler{m.p}.hold()
}

// SetupSystems checks the bindings once everything is installed: a command nobody listens to
// is a configuration error, reported with its type and label.
func (m *module) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(*goke.SysInit) {
		var missing []string
		for _, pl := range m.p.players {
			for _, b := range pl.bindings {
				if !m.p.worldPlugin.Carrier().Takes(b.Command()) {
					missing = append(missing, fmt.Sprintf("%v (%q for %s)", b.Command(), b.Label, pl.Name))
				}
			}
		}
		if len(missing) > 0 {
			panic("players: no plugin listens for " + strings.Join(missing, ", "))
		}
	}}}
}

// LoadComps is empty — players own no components.
func (m *module) LoadComps() []goke.CompToken { return nil }
