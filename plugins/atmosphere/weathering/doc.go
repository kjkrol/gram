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
package weathering
