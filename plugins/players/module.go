package players

import (
	"fmt"
	"strings"
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

// module runs the camera system and, once every plugin is installed, checks that each bound
// command has an owner.
type module struct {
	p        *Plugin
	runnable goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) { m.runnable = ecs.RegSys(&cameraSystem{p: m.p}) }

// RunPlan moves the cameras and empties every queue, then issues the KeyHeld commands of the keys
// still down, for the next tick; call it after the plugins that drain theirs.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
	for _, box := range m.p.queues {
		box.Clear()
	}
	translator{m.p}.hold()
}

// SetupSystems checks the bindings once everything is installed: a command nobody listens to
// is a configuration error, reported with its type and label.
func (m *module) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(*goke.SysInit) {
		var missing []string
		for _, pl := range m.p.players {
			for _, b := range pl.bindings {
				if _, ok := m.p.queues[b.Command()]; !ok {
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
