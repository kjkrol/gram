// Package steering turns what an entity wants into motion, gradually: a [Steering] carries the
// heading asked for and a motion profile — the top speed, how fast it speeds up and brakes, the
// speed it sets off at — and the [System], run by the world in every step of its simulation before
// movement, turns the entity's heading towards the one wanted by at most TurnRate a tick and
// writes its base speed from the profile. [Steering.Request] asks for a heading (after Reflex
// ticks), [Steering.RequestSpeed] for a speed. Navigation steers units through it; a game's
// behaviours may as well.
//
// [Driven] marks an entity steered by hand — walk on or stop, turn, or turn to face a way —
// written every tick by whoever steers it (a camera riding in it) and carried out by the plugin
// that moves entities over the ground (plugins/navigation).
package steering
