// Package water works out the running water of a relief: where the rain on it gathers and runs
// to the sea, the brooks, streams and rivers it makes, the channels they cut and the fords across
// them. It is a way to make a board's layout, not a plugin: a game drains its heights once with
// [Drain], lays the courses on the board as a network of its own kinds ([Network.Net]) and seeds
// the board with the [Network.Carved] heights.
//
// # Draining
//
// [Drain] floods the relief from the sea up, so every cell drains to the sea and a hollow fills
// until it spills; each cell's level is nudged by [Config.Meander], so courses wander. The rain
// ([Config.Rain]) gathers downstream: where enough has gathered a cell is a [Brook], a [Stream]
// or a [River], as wide as the square root of its water says; every [Config.FordEvery] cells of a
// gently running river a [Ford] crosses it. A course reaching the sea runs on out into it as a
// [Mouth] on each cell for [Config.Plume] cells, wider and more faded ([Network.Fade]) each cell.
//
// # Carving
//
// A course's bed lies [Config.BrookDepth], [Config.StreamDepth] or [Config.RiverDepth] below the
// level its cell was filled to; [Network.Carved] puts every corner of its cell at the bed, so a
// channel is a valley, never a trench. Only a square grid has corners: Drain refuses any other
// with [ErrNotSquare].
package water
