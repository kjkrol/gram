// Package entity is what every entity of a world carries: its [Base] — where it is ([Position]),
// how it moves ([Velocity]), the kind it was spawned from and what the space may do with it — its
// planes ([Layers]) and, in a world with heights, its place in height ([Z]). No entity moves
// further in a tick than [StepReach] of its own shorter side.
//
// The package is a leaf under plugins/world, so the world's sub-packages — steering, view — can
// read the components without importing the world; the world re-exports them, so a game and the
// other plugins say world.Base, world.Position, world.Velocity, world.Z and world.Layers as ever.
package entity
