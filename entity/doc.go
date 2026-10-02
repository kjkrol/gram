// Package entity is what every entity of a world carries: its [Base] — where it is ([Position]),
// how it moves ([Velocity]), the kind it was spawned from and what the space may do with it — its
// planes ([Layers]), in a world with heights its place in height ([Z]), and, for one that looks,
// where it looks from and how wide ([Eye]). No entity moves further in a tick than [StepReach]
// of its own shorter side.
//
// The package is gram's core, imported by no plugin it needs, so whatever reads the components —
// the world's steering and view, every plugin — does so without importing the world; the world
// re-exports them, so a game and the plugins say world.Base, world.Position, world.Velocity,
// world.Z and world.Layers as ever.
package entity
