// Package steering turns what an entity wants into motion, gradually. A [Steering] is how it may
// be steered — how fast it turns and answers, and a motion profile: the top speed, how fast it
// speeds up and brakes, the speed it sets off at; Halted holds it where it is. It is a knob the
// steering only reads, which an effect may Alter. What the entity is asked and how far it has come
// is its [Course], beside it — the world gives one to every unit, the [System] to an entity that
// has none. Whoever steers holds both as a [Helm]: [Helm.Request] asks for a heading (after
// Reflex ticks), [Helm.RequestSpeed] for a speed, [Helm.RequestBack] to back away and
// [Helm.RequestSprint] for Sprint times the top speed, a hand urging it on. The System, run by the
// world in every step of its simulation before movement, turns the entity's heading towards the
// one wanted by at most TurnRate a tick and writes its base speed from the profile. Navigation
// steers units through it.
//
// [Driven] marks an entity steered by hand — walk on, sprint or stop, turn, or turn to face a way,
// and, flown from inside, how steeply to climb where it flies ([Driven.Slope]) — written every tick
// by whoever steers it (a camera riding in it) and carried out by the plugin that moves entities
// over the ground (plugins/navigation) and the one that knows the ground's height
// (plugins/topography).
package steering
