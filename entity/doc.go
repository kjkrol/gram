// Package entity is what every entity of a world carries: its [Base] — where it is ([Position]),
// how it moves ([Velocity]), the kind it was spawned from and what the space may do with it — its
// planes ([Layers]), in a world with heights its place in height ([Z]), and, for one that looks,
// where it looks from and how wide ([Eye]). No entity moves further in a tick than [StepReach]
// of its own shorter side.
//
// # Names and groups
//
// An entity may be called something: its [Label] holds a name, its alone, and a group it is in
// with others — a unit's from its kind.Entry (Named, InGroup), a cell's from its cell.Entry. A
// command says whom it is for with a [Whom]: [Named], the entities bearing those names, [Group],
// all those in the groups, or [World], the world's own entity — rule.Cast(open).On(entity.Group(
// "trapdoors")). The world finds them by their Labels.
//
// The package is gram's core, imported by no plugin it needs, so whatever reads the components —
// the world's steering and view, every plugin — does so without importing the world; the world
// re-exports them, so a game and the plugins say world.Base, world.Position, world.Velocity,
// world.Z and world.Layers as ever.
package entity
