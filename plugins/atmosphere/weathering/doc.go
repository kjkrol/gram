// Package weathering is the weather on the board: snow lying, ice on the water, what sways
// swaying, as effects on the board's cells cast as the world's weather says and taken off again.
//
// [New] takes the board, the world, the world's effects, the calendar and the [Config]: which of
// the game's kinds a kind becomes under snow (the game makes both kinds — the snowy one the same
// ground with another look), what water freezes into, what sways in the wind, where snow lies
// first (high ground), and the seed of its dice. It defines three effects: snow turns a cell's
// kind into its snowy one, ice turns water into ice, sway has a kind bend in the wind. Laid on
// the world's schedule ([Weathering.Schedule]) it works once a second of game time — hurried with
// the tempo, stopped in the tactical pause: snow settles on cells here and there while it snows in
// the frost, in patches that grow from the drifts' pattern, high ground and snow already lying,
// and melts off them once it is warm, the loneliest first; water freezes from the shore out in a
// hard frost and thaws; what sways sways while the wind blows hard. A winter begun has its snow
// and its shores' ice at once. The atmosphere plugin lays it on (atmosphere.Plugin.WithWeathering).
package weathering
