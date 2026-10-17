// Package weathering is the weather on the board: snow lying, ice on the water, what sways
// swaying, as effects on the board's cells cast as the world's weather says and taken off again.
//
// # Weathering
//
// [New] takes the board, the weather, the world's effects, the calendar and a [Config] naming the
// game's own kinds: what a kind becomes under snow, what water freezes into, what sways in the
// wind, where snow lies first. It defines three effects, snow, ice and sway
// ([Weathering.Effects]). [Weathering.System], run in every step of the simulation, works once a
// second of game time: snow settles in patches where it snows in the frost and melts once it is
// warm, water freezes from the shore out in a hard frost and thaws, what sways sways while the
// wind blows hard; a winter begun has its snow and its shores' ice at once. The atmosphere plugin
// lays it on (atmosphere.Plugin.WithWeathering).
//
// # Land
//
// [Weathering.System] walks the board's cells too, a share a step, all of them a second of game
// time: their ground and their [Wring] noted on the [Weathering.Land] the climate works on the air
// with (air.Dome.Lift), so the ground holds back the low clouds that run into it.
//
// # Overhead
//
// Every cell hears an [Overhead] once a second: the air over it, its low clouds, how high its
// ground reaches into them, the time of day ([Weathering.Rules]; its words [Clouded], [Damp],
// [Reaches] and [Hours]). A rule of a role the cell plays puts on an effect turning its [Wring]:
// the rain the low clouds over it let fall, the damp gathering into clouds over it. The weathering
// ships no rule of its own.
//
// # Vocabulary
//
// Moments, for a role's rules: [Overhead] (a cell's entity, once a second; its words [Clouded],
// [Damp], [Reaches], [Hours]). Knobs, for an effect's Alter: [Wring] (on every cell).
package weathering
