// Package water works out the running water of a relief: where the rain on it gathers and runs
// to the sea, the brooks, streams and rivers it makes, the channels they cut and the fords across
// them. It is a way to make a board's layout, as relief.MeanOfCells is, not a plugin: a game
// drains its heights once, lays its courses as a network (plugins/board/network) of its own kinds
// — [Network.Net]: a node on every cell [Network.Courses] names, [Network.Width] wide, each
// flowing down to where its water goes — and seeds the board with the [Network.Carved] heights.
//
// # Draining
//
// [Drain] floods the relief from the sea up, cell by cell, the lowest first, over every neighbour
// of a cell, across the corners too, so water runs slantwise down a slanting valley; each cell's
// level is nudged a little for it ([Config.Meander]), so courses wander instead of running straight
// down an even slope — less and less within a few cells of the sea, so a course near it runs
// straight for it instead of along the shore into another. Each cell's
// water goes down to the cell that reached it, and a hollow is filled until it spills: every cell
// drains to the sea. The rain ([Config.Rain], one a cell by default) gathers downstream; where
// enough has gathered it is a [Brook], where more a [Stream], where more still a [River]. On a
// river, every [Config.FordEvery] cells from its mouth where it runs gently, a [Ford] crosses it.
// A course runs down to where its water goes and up to every course draining into it; it is the
// wider the more water it gathers, by the square root of it, a cell wide at most. A walker never
// slips between two cells of a river meeting at a corner: the board's planner keeps a step
// across a corner to cells it may enter on both sides. A course reaching the sea runs on out into
// it ([Config.Plume] cells by the square root of its water, the way of its last step): a [Mouth]
// on each cell of the sea, wider than the last and more faded ([Network.Fade]), the biggest
// course's first where two would take the same sea. Laid as a network, how far down its course a
// cell lies (network.Network.Along) turns a river into the sea's look as it nears it.
//
// # Carving
//
// A course's bed lies [Config.BrookDepth], [Config.StreamDepth] or [Config.RiverDepth] below the
// level its cell was filled to, and falls all the way to the sea. [Network.Carved] puts every
// corner of a course's cell at its bed: the corners are shared with the banks, so a channel is a
// valley, never a trench with walls, and where the bed drops off a cliff the water falls. Corners
// at the sea stay as they are. A grid other than square has no corners to carve: Drain refuses
// it.
package water
