// Package effect puts states on entities for a while — each effect with its own marker, tags
// granted, components altered and later restored. An effect is defined once, from a Spec of
// traits, and cast from anywhere — a rule or a plan through Apply, Keep and Dispel
// (package rule), a key, another plugin — its handle ([Effect]) casting on the effects it
// came from. An effect's presence is state: what a rule remembers, what a plan asks with Unless,
// what rules of other plugins filter by. The world makes and runs the effects
// (world.Plugin.Effects), in its simulation, so they count down in the tactical clock's time: they
// stand in the pause and go with the tempo.
//
// # Spec, the marker, Grant and Alter
//
// [Effects.Define] registers an effect from a [Spec] under a name — it hands nothing back, and
// [Effects.Named] is the effect wherever it is built on — and gives it its own marker of [States],
// named "effect.<name>" and saved by name: on while the effect runs, off when the last of its
// casts ends — [Effect.Mark], for rule.Self(burning.Mark()) in any plugin. [Lasts] is how long a
// cast holds (without it, until Dispel); [Stacking] lets casts pile up instead of refreshing;
// [Then] casts another effect when this one's time is up — not when it is dispelled; [Grant] gives
// tags of a game's family while the effect runs and takes them back after, unless another running
// effect grants them; [Alter] changes a component and restores the original after — several
// Alters of one component stack from the original in slot order, whichever ends first. The plugin
// keeps the originals itself and saves them with the game. At most 63 effects are defined, one
// marker each beside Changed. [Described] gives the effect a sentence for a UI — a tooltip —
// handed back by [Effect.Description].
//
// # Active, Cast and Dispel
//
// [Active] is what an entity is under: up to [maxEffects] slots, saved with it; [Wide] is the
// same with a slot for every effect a game may define, what a plugin's own entity carries
// (world.Self). [Effects.Cast]
// and [Effects.CastFor] put an effect on an entity — attaching Active when it has none — and the
// change lands with the effects' next pass; a cast after a Dispel in the same step takes the slot
// back. [Effects.Dispel] ends one with that pass; [Effects.Has] asks; the handle's [Effect.Cast],
// [Effect.CastFor], [Effect.Dispel] and [Effect.On] do the same.
//
// # Changed
//
// An entity keeps its Active once an effect came, empty when none runs — a component put on and
// taken off would move the entity in memory each time. Its markers ([States], carried for good:
// the world gives them to every unit, the board to every cell, an entity without them gets them
// at its first effect) have [Changed] on for the step after an Alter rewrote one of its components
// — as the effect began or as it ended — so a plugin owning that component, the board with a
// cell's ground, learns of the change without keeping a copy to compare. An effect something is
// drawn by ([Effect.Shows]: a board's cover) turns it on as it begins and ends, altering nothing.
package effect
