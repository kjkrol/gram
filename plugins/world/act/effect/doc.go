// Package effect puts temporary changes on entities — a tag granted for a while, a component
// altered and later restored — and fires the triggers of the clock's moments. An effect is
// defined once, from a Spec of traits, and cast from anywhere — a trigger or a tree through
// Apply and While (plugins/world/act), a key, another plugin — its handle ([Effect]) casting on the
// effects it came from. An effect's presence is state: what a trigger remembers, what a tree
// asks with Unless. The world makes and runs the effects (world.Plugin.Effects), in its
// simulation, so they count down in the tactical clock's time: they stand in the pause and go
// with the tempo.
//
// # Spec, Grant and Alter
//
// [Effects.Define] registers an effect from a [Spec]: [Lasts] is how long a cast holds (without it,
// until [Dispel]); [Stacking] lets casts pile up instead of refreshing; [Grant] gives tags of a
// family while the effect runs and takes them back after, unless another running effect grants
// them; [Alter] changes a component and restores the original after — several Alters of one
// component stack from the original in slot order, whichever ends first. The plugin keeps the
// originals itself and saves them with the game.
//
// # Active, Cast and Dispel
//
// [Active] is what an entity is under: up to [MaxEffects] slots, saved with it. [Effects.Cast]
// and [Effects.CastFor] put an effect on an entity — attaching Active when it has none — and the
// change lands with the effects' next pass; [Effects.Dispel] ends one early; [Effects.Has] asks;
// the handle's [Effect.Cast], [Effect.CastFor], [Effect.Dispel] and [Effect.On] do the same.
// Active.Altered says the last pass rewrote one of the entity's components through an Alter, so a
// plugin owning that component learns of the change without keeping a copy to compare.
//
// # Idle
//
// An entity keeps its Active once an effect came, empty when none runs — a component put on and
// taken off would move the entity in memory each time. Its markers ([States], carried for good:
// the world gives them to every unit, the board to every cell) have [Idle] on for the one step
// after its last effect ended: the board counts a cell it finds so as changed back, and a trigger
// of an [Idling] hooked on the world hears of it once — a life lost when the shield ends.
//
// # The clock's moments
//
// Effects fire the triggers of the clock's Moment once every step of the simulation, with their
// pass: what happens when — clock.At a time, clock.Every period after an offset, a calendar's
// every day at an hour or every winter — is a act.Trigger[clock.Moment] holding one of them,
// hooked on the world. Casting an effect that grants a clock.Phase tag onto the clock's entity
// switches a phase on until the effect ends, for a trigger to run only while it holds
// (clock.Clock.In). Triggers fire by the clock's time alone, so a loaded game goes on from where
// it was.
package effect
