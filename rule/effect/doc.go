// Package effect puts states on entities for a while: each effect with its own marker, tags
// granted, components altered and restored. An effect is defined once, from a [Spec] of traits,
// and cast from anywhere — a rule's act.Self.Put, Hold and Take, a command, another plugin.
// The world makes and runs the effects (world.Plugin.Effects) in its simulation, so they count
// down in game time.
//
// # Spec, the marker, Grant and Alter
//
// [Effects.Define] registers an effect from a Spec under a name and [Effects.Named] is the
// effect wherever it is built on. Every effect has its own marker of [States], "effect.<name>",
// on while it runs ([Effect.Mark]; a cond clause of the effect, cond.Self.If(e), asks it in any
// plugin). [Lasts] is how long a cast
// holds (without it, until Dispel); [Stacking] lets casts pile up instead of refreshing; [Then]
// casts another effect when this one's time is up, not when it is dispelled; [Grant] gives tags
// of a game's family while the effect runs; [Alter] changes a component and restores the
// original after, several Alters of one component stacking in slot order; [Described] gives it a
// sentence for a UI. At most 63 effects are defined.
//
// # Active, Cast and Dispel
//
// [Active] is what an entity is under, saved with it; [Wide] is the same with a slot for every
// effect, what a plugin's own entity carries. [Effects.Cast] and [Effects.CastFor] put an effect
// on an entity, landing with the effects' next pass; [Effects.Dispel] ends one; [Effects.Has]
// and the handle's [Effect.On] ask.
//
// # Changed
//
// An entity's markers have [Changed] on for the step after an Alter rewrote one of its
// components, as the effect began or ended, so a plugin owning that component learns of the
// change without keeping a copy to compare. An effect something is drawn by ([Effect.Shows])
// turns it on as it begins and ends, altering nothing.
package effect
