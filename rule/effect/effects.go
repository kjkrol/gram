package effect

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/uid"
)

// Effects puts effects on entities — temporary changes to their components: tags granted for a
// while, values altered and restored — cast from anywhere. The world makes and runs it
// (world.Plugin.Effects); nothing installs it by hand.
type Effects struct {
	defs      []def
	originals *originals
	name      func(name string) tag.Tag[States]
	system    *effectSystem
	module    *Module
}

// New makes the effects, naming each effect's marker with name — the world's Kinds, so the saves
// know it by name.
func New(name func(name string) tag.Tag[States]) *Effects {
	e := &Effects{originals: newOriginals(), name: name}
	e.system = newEffectSystem(&e.defs, e.originals)
	e.module = &Module{system: e.system, originals: e.originals}
	return e
}

// markerPrefix comes before an effect's name in the name of its marker: "effect.burning".
const markerPrefix = "effect."

// Define registers an effect under name, with its own marker on while it runs (Effect.Mark);
// call it in Init, before the game runs.
func (e *Effects) Define(name string, spec Spec) Effect {
	if e.system.built {
		panic(fmt.Sprintf("effects: %q defined after the game was set up", name))
	}
	if len(e.defs) == tag.MaxTagsPerFamily-1 {
		panic(fmt.Sprintf("effects: %q is one too many: at most %d effects, each with its marker", name, tag.MaxTagsPerFamily-1))
	}
	mark := e.name(markerPrefix + name)
	if mark == Changed {
		panic(fmt.Sprintf("effects: %q is named as Changed is", name))
	}
	d := def{name: name}
	Grant(mark).apply(&d)
	for _, t := range spec {
		t.apply(&d)
	}
	e.defs = append(e.defs, d)
	return Effect{owner: e, id: ID(len(e.defs) - 1), mark: mark}
}

// Cast puts effect on id for as long as its Spec says; cast again, it is refreshed unless it
// stacks. The change lands with the effects' next pass.
func (e *Effects) Cast(cb *goke.CmdBuf, id uid.UID64, effect Effect) {
	e.CastFor(cb, id, effect, e.lasts(effect.id))
}

// CastFor is Cast for a set time of the clock's; Forever for one that lasts until Dispel.
func (e *Effects) CastFor(cb *goke.CmdBuf, id uid.UID64, effect Effect, d time.Duration) {
	e.system.cast(cb, id, effect.id, d)
}

// Dispel ends effect on id with the effects' next pass; nothing happens without it.
func (e *Effects) Dispel(id uid.UID64, effect Effect) { e.system.dispel(id, effect.id) }

// Has reports whether id is under effect.
func (e *Effects) Has(id uid.UID64, effect Effect) bool { return e.system.has(id, effect.id) }

func (e *Effects) lasts(effect ID) time.Duration { return lastsOf(&e.defs[effect]) }

// lastsOf is how long a cast of d lasts when nobody says: its Lasts, or Forever.
func lastsOf(d *def) time.Duration {
	if d.lasts > 0 {
		return d.lasts
	}
	return Forever
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

// RunPlan advances every effect over d — a step of the simulation.
func (m *Module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

// SetupSystems is empty — effects are cast at runtime.
func (m *Module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types effects owns — see [goke.CompProvider].
func (m *Module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Active](), goke.LoadComp[tag.Tags[States]]()}
}

// Persisted returns the saved originals for Persistence.Save and Load.
func (m *Module) Persisted() []any { return []any{&m.originals.byEntity} }

// Effect is one effect defined with its Effects: what a plan or a rule casts, the Effects it
// belongs to carried along.
type Effect struct {
	owner *Effects
	id    ID
	mark  tag.Tag[States]
}

// ID is the effect's number, as Active's slots keep it.
func (e Effect) ID() ID { return e.id }

// Mark is the effect's own marker, on while it runs: what rules of other plugins filter by —
// rule.Self(burning.Mark()).
func (e Effect) Mark() tag.Tag[States] { return e.mark }

// Cast puts the effect on id for as long as its Spec says; see Effects.Cast.
func (e Effect) Cast(cb *goke.CmdBuf, id uid.UID64) { e.owner.Cast(cb, id, e) }

// CastFor puts the effect on id for d of the clock's time; Forever until Dispel.
func (e Effect) CastFor(cb *goke.CmdBuf, id uid.UID64, d time.Duration) {
	e.owner.CastFor(cb, id, e, d)
}

// Dispel ends the effect on id with the effects' next pass.
func (e Effect) Dispel(id uid.UID64) { e.owner.Dispel(id, e) }

// On reports whether id is under the effect.
func (e Effect) On(id uid.UID64) bool { return e.owner.Has(id, e) }
