// Package effects puts temporary changes on entities — a tag granted for a while, a component
// altered and later restored — and keeps the schedule of what happens when. An effect is defined
// once, from a Spec of traits, and cast from anywhere — a behavior, a key, another plugin. The
// world makes and runs the effects (world.Plugin.Effects), in its simulation, so they count down
// in the tactical clock's time: they stand in the pause and go with the tempo.
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
// change lands with the effects' next pass; [Effects.Dispel] ends one early; [Effects.Has] asks.
// Active.Altered says the last pass rewrote one of the entity's components through an Alter, so a
// plugin owning that component learns of the change without keeping a copy to compare.
//
// # Idle
//
// An entity whose last effect ended loses its Active and carries [Idle] for one tick: the board
// counts a cell it finds so as changed back, and an [Each] or [Every] of an [Idling] registered
// with the world's RegisterBehavior hears of it once — a life lost when the shield ends. Effects
// host nothing else: they are what behaviors cast.
//
// # Schedule
//
// [Effects.Schedule] is what happens when: an entry [Schedule.At] a moment of the clock's time, or
// [Schedule.Every] period after an offset — a calendar's every day at an hour, every winter —
// runs what it was given in the step of the simulation its moment falls in, with a Tick to cast
// with. Casting an effect that grants a clock.Phase tag onto the clock's entity switches a phase
// on until the effect ends, for a behavior to run only while it holds (clock.Clock.In). Entries are
// laid in code at Init and fire by the clock's time alone, so a loaded game goes on from where it
// was.
package effects
