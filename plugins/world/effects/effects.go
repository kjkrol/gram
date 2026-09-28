package effects

import (
	"errors"
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world/clock"
	"github.com/kjkrol/uid"
)

// Effects puts effects on entities — temporary changes to their components: tags granted for a
// while, values altered and restored — cast from anywhere, and keeps the Schedule of what happens
// when. The world makes and runs it (world.Plugin.Effects); nothing installs it by hand.
type Effects struct {
	defs      []def
	originals *originals
	idlers    host.EachHost[Idling]
	schedule  Schedule
	system    *effectSystem
	module    *Module
}

// New makes the effects over clk, whose time they count down in.
func New(clk *clock.Clock) *Effects {
	e := &Effects{originals: newOriginals()}
	e.schedule.clock = clk
	e.system = newEffectSystem(&e.defs, e.originals, &e.idlers, &e.schedule)
	e.module = &Module{system: e.system, originals: e.originals}
	return e
}

// Define registers an effect under name; call it in Init, before the game runs.
func (e *Effects) Define(name string, spec Spec) ID {
	if e.system.built {
		panic(fmt.Sprintf("effects: %q defined after the game was set up", name))
	}
	if len(e.defs) == 1<<8 {
		panic("effects: at most 256 effects")
	}
	d := def{name: name}
	for _, t := range spec {
		t.apply(&d)
	}
	e.defs = append(e.defs, d)
	return ID(len(e.defs) - 1)
}

// Cast puts effect on id for as long as its Spec says; cast again, it is refreshed unless it
// stacks. The change lands with the effects' next pass.
func (e *Effects) Cast(cb *goke.CmdBuf, id uid.UID64, effect ID) {
	e.CastFor(cb, id, effect, e.lasts(effect))
}

// CastFor is Cast for a set time of the clock's; Forever for one that lasts until Dispel.
func (e *Effects) CastFor(cb *goke.CmdBuf, id uid.UID64, effect ID, d time.Duration) {
	e.system.cast(cb, id, effect, d)
}

// Dispel ends effect on id with the effects' next pass; nothing happens without it.
func (e *Effects) Dispel(id uid.UID64, effect ID) { e.system.dispel(id, effect) }

// Has reports whether id is under effect.
func (e *Effects) Has(id uid.UID64, effect ID) bool { return e.system.has(id, effect) }

// Schedule is what happens when, on the clock's time.
func (e *Effects) Schedule() *Schedule { return &e.schedule }

func (e *Effects) lasts(effect ID) time.Duration {
	if d := e.defs[effect].lasts; d > 0 {
		return d
	}
	return Forever
}

// Host takes an Each or Every of Idling — run once for an entity whose last effect ended —
// for the world's RegisterBehavior; ErrUnhostedBehavior for anything else.
func (e *Effects) Host(b plugin.Behavior) error {
	if err := e.idlers.Add(b); err != nil {
		if errors.Is(err, plugin.ErrUnhostedBehavior) {
			return err
		}
		return fmt.Errorf("%w in effects", err)
	}
	return nil
}

// Module is the effects as a goke.Module, for the world to install: its system, its components
// and the originals it saves.
func (e *Effects) Module() *Module { return e.module }

// Module runs the effect system, lists its components and saves the originals of what is altered.
type Module struct {
	system    *effectSystem
	originals *originals
	runnable  goke.Runnable
}

var _ goke.Module = (*Module)(nil)
var _ plugin.Serializable = (*Module)(nil)

// RegSystems registers the effect system once, however many times the module is registered: the
// world registers it with its own systems and installs it as a module of its own.
func (m *Module) RegSystems(ecs *goke.ECS) {
	if m.runnable == nil {
		m.runnable = ecs.RegSys(m.system)
	}
}

// RunPlan advances every effect and the schedule over d — a step of the simulation.
func (m *Module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

// SetupSystems is empty — effects are cast at runtime.
func (m *Module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types effects owns — see [goke.CompProvider].
func (m *Module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Active](), goke.LoadComp[Idle]()}
}

// Persisted returns the saved originals for Persistence.Save and Load.
func (m *Module) Persisted() []any { return []any{&m.originals.byEntity} }
