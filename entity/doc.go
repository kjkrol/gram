// Package entity is what every entity of a world carries: its [Base] (where it is, [Position];
// how it moves, [Velocity]; its kind and what the space may do with it), its planes ([Layers]),
// in a world with heights its place in height ([Z]), where it looks from ([Eye]), the [Drift] it
// is carried at under gravity, how it lies flat ([Lie]) and how it casts its shadow ([Shadow]).
// No entity moves further in a tick than [StepReach] of its own
// shorter side.
//
// # Names and groups
//
// An entity may be called something: its [Label] holds a name, its alone, and a group it is in
// with others. A command says whom it is for with a [Whom]: [Named], the entities bearing those
// names, [Group], all those in the groups, [ID], the entities of those identifiers, or [World],
// the world's own entity.
//
// The package is gram's core, imported by no plugin it needs; the world re-exports its
// components, so a game and the plugins say world.Base, world.Position and kin.
package entity
