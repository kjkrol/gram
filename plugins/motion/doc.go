// Package motion moves the world's entities: every step it steers them, runs the rules of a
// [Moving] over them, moves each by its velocity, never further than Position.MaxStep, and deals
// with those gone past an open edge. A world without it keeps its entities where they stand.
//
// # Plugin
//
// [Plugin] is made over the world ([NewPlugin]) before what steers or pushes the entities, which
// takes it as it is made (collision, bullet, driving, navigation, gravity; a board's units,
// board.Plugin.WithMotion); its [Plugin.RunPlan] is called right after the world's. Its
// sub-package steering is how an entity is steered: a heading and a speed reached gradually.
//
// # Units and Drift
//
// [UnitTemplate] is the template of the kinds that move, under the world's entity template: the
// plugins ask of it what every unit carries, motion a steering.Course. [Drift] is a velocity an
// entity is carried at beside its own, which gravity writes.
//
// # Leaving
//
// An entity wholly past an open edge carries [Outside]; the rules of a [Leaving] hear of it every
// tick, and with none it is despawned. A plugin moving entities itself marks one it put out with
// [Plugin.Leave].
//
// # Vocabulary
//
// Moments, for a role's rules: [Moving] (every entity before it moves; its words [Still],
// [Braking]), [Leaving] (every tick an entity is past an open edge). Knobs, for an effect's
// Alter: steering.Steering. Commands, an entity's own: steering.Away, steering.Toward,
// steering.Turn.
package motion
