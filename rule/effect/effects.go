package effect

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/render"
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
	module    *module

	byName  map[string]effectID
	marks   []tag.Tag[States]                     // by effect: its own marker
	sprite  func() render.SpriteID                // the world's issuer of atlas slots
	looks   []map[render.SpriteID]render.SpriteID // by effect: a sprite's twin under it
	settled bool                                  // the world took the looks for its renderer
	guard   func(name string)                     // panics for an effect defined out of its place; nil for none
}

// New makes the effects, naming each effect's marker with name — the world's Kinds, so the saves
// know it by name; name gives Changed first. For the world, which makes the game's.
func New(name func(name string) tag.Tag[States]) *Effects {
	if t := name(changedName); t != Changed {
		panic(fmt.Sprintf("effects: the markers have tags of their own before %q", changedName))
	}
	e := &Effects{originals: newOriginals(), name: name}
	e.system = newEffectSystem(&e.defs, e.originals)
	e.module = &module{system: e.system, originals: e.originals}
	return e
}

// markerPrefix comes before an effect's name in the name of its marker: "effect.burning".
const markerPrefix = "effect."

// Define registers an effect under name, with its own marker on while it runs (Effect.Mark);
// call it in Init, before the game runs. It hands nothing back: Named is the effect, for whoever
// builds on it.
func (e *Effects) Define(name string, spec Spec) {
	if e.guard != nil {
		e.guard(name)
	}
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
	e.looks = append(e.looks, nil)
	e.marks = append(e.marks, mark)
	id := effectID(len(e.defs) - 1)
	if e.byName == nil {
		e.byName = map[string]effectID{}
	}
	e.byName[name] = id
}

// Named is the effect defined as name, for whoever builds on it in Init or in a scene's Layers —
// rules, looks, commands; a game keeps its names as constants, and what runs keeps the Effect
// itself, never the name. An unknown name panics.
func (e *Effects) Named(name string) Effect {
	id, ok := e.byName[name]
	if !ok {
		panic(fmt.Sprintf("effects: no effect is defined as %q", name))
	}
	return Effect{owner: e, id: id, mark: e.marks[id]}
}

// Guard has Define call guard with each effect's name first: the world's, which refuses one
// defined out of its section of a Stage. For the world.
func (e *Effects) Guard(guard func(name string)) { e.guard = guard }

// Sprites has the effects' looks take their atlas slots from issue. For the world.
func (e *Effects) Sprites(issue func() render.SpriteID) { e.sprite = issue }

// Looks calls each with every effect that has looks — its marker and the twins of the sprites
// drawn under it — and settles which effects have any. For the world's renderer.
func (e *Effects) Looks(each func(mark tag.Tag[States], twins map[render.SpriteID]render.SpriteID)) {
	e.settled = true
	for id, twins := range e.looks {
		if twins != nil {
			each(e.marks[id], twins)
		}
	}
}

// Cast puts effect on id for as long as its Spec says; cast again, it is refreshed unless it
// stacks. The change lands with the effects' next pass. For plugins: a game casts by a rule's or a
// plan's Apply.
func (e *Effects) Cast(cb *goke.CmdBuf, id uid.UID64, effect Effect) {
	e.CastFor(cb, id, effect, e.lasts(effect.id))
}

// CastFor is Cast for a set time of the clock's; Forever for one that lasts until Dispel.
func (e *Effects) CastFor(cb *goke.CmdBuf, id uid.UID64, effect Effect, d time.Duration) {
	e.system.cast(cb, id, effect.id, d)
}

// Dispel ends effect on id with the effects' next pass; nothing happens without it. For plugins.
func (e *Effects) Dispel(id uid.UID64, effect Effect) { e.system.dispel(id, effect.id) }

// Has reports whether id is under effect. For plugins.
func (e *Effects) Has(id uid.UID64, effect Effect) bool { return e.system.has(id, effect.id) }

func (e *Effects) lasts(effect effectID) time.Duration { return lastsOf(&e.defs[effect]) }

// lastsOf is how long a cast of d lasts when nobody says: its Lasts, or Forever.
func lastsOf(d *def) time.Duration {
	if d.lasts > 0 {
		return d.lasts
	}
	return Forever
}

// Module is the effects as a goke.Module, for the world to install: its system, its components
// and the originals it saves.
func (e *Effects) Module() goke.Module { return e.module }

// module runs the effect system, lists its components and saves the originals of what is altered.
type module struct {
	system    *effectSystem
	originals *originals
	runnable  goke.Runnable
}

var _ goke.Module = (*module)(nil)

// RegSystems registers the effect system once, however many times the module is registered: the
// world registers it with its own systems and installs it as a module of its own.
func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable == nil {
		m.runnable = ecs.RegSys(m.system)
	}
}

// RunPlan advances every effect over d — a step of the simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

// SetupSystems is empty — effects are cast at runtime.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types effects owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Active](), goke.LoadComp[Wide](), goke.LoadComp[tag.Tags[States]]()}
}

// Persisted returns the saved originals for Persistence.Save and Load.
func (m *module) Persisted() []any { return []any{&m.originals.byEntity} }

// Effect is one effect defined with its Effects: what a plan or a rule casts, the Effects it
// belongs to carried along.
type Effect struct {
	owner *Effects
	id    effectID
	mark  tag.Tag[States]
}

// Look is the atlas slot drawn under the effect in place of the sprite of: issued the first time
// it is asked for, the same after. Register what it shows in the world's atlas; the world's
// renderer swaps it in while the effect's marker is on. An effect's first look comes before the
// world's renderer is made (world.Plugin.WithRenderer), or it panics.
func (e Effect) Look(of render.SpriteID) render.SpriteID {
	o := e.owner
	twins := o.looks[e.id]
	if id, ok := twins[of]; ok {
		return id
	}
	if o.sprite == nil {
		panic(fmt.Sprintf("effects: %q has no looks: these effects issue no sprites", o.defs[e.id].name))
	}
	if twins == nil {
		if o.settled {
			panic(fmt.Sprintf("effects: the first look of %q comes after the world's renderer was made", o.defs[e.id].name))
		}
		twins = map[render.SpriteID]render.SpriteID{}
		o.looks[e.id] = twins
	}
	id := o.sprite()
	twins[of] = id
	return id
}

// Shows says something is drawn by the effect's marker — a cover on a board's cells: the entity's
// Changed goes on as the effect begins and ends, though it alters nothing. For plugins.
func (e Effect) Shows() { e.owner.defs[e.id].shows = true }

// Mark is the effect's own marker, on while it runs: what rules of other plugins filter by —
// rule.Self(burning.Mark()).
func (e Effect) Mark() tag.Tag[States] { return e.mark }

// Description is what the Spec's Described said of the effect — for a UI, a tooltip; "" without one.
func (e Effect) Description() string { return e.owner.defs[e.id].describe }

// On reports whether id is under the effect. For plugins.
func (e Effect) On(id uid.UID64) bool { return e.owner.Has(id, e) }
