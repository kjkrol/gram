// Package water works out the running water of a relief: where the rain on it gathers and runs
// to the sea, the streams and rivers it makes, the channels they cut and the fords across them.
// It is a way to make a board's layout, as board.MeanOfCells is, not a plugin: a game drains its
// heights once, lays its own kinds where [Network.Courses] says, and seeds the board with
// [Network.Carved] heights.
//
// # Draining
//
// [Drain] floods the relief from the sea up, cell by cell, the lowest first, over the four
// neighbours of a square grid (never across a corner, so a river is whole and nothing slips
// between two of its cells). Each cell's water goes down to the cell that reached it, and a hollow
// is filled until it spills: every cell drains to the sea. The rain ([Config.Rain], one a cell by
// default) gathers downstream; where enough has gathered it is a [Stream], where more a [River],
// and where more still the river is two cells wide. On a river, every [Config.FordEvery] cells
// from its mouth where it runs gently, a [Ford] crosses it.
//
// # Carving
//
// A course's bed lies [Config.StreamDepth] or [Config.RiverDepth] below the level its cell was
// filled to, and falls all the way to the sea. [Network.Carved] puts every corner of a course at
// its bed: the corners are shared with the banks, so a channel is a valley, never a trench with
// walls, and where the bed drops off a cliff the water falls. Corners at the sea stay as they are.
// A grid other than square has no corners to carve: Drain refuses it.
package water
