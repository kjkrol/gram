// Package relief is the ground's heights under a board: how high every corner stands, the ground
// between them, what climbing it costs, how it is shaped, and the heights saved with the game.
//
// A [Relief] ([New]) holds a height at every corner of a square grid's lattice — a cell's four
// corners ([Corners]) — or at every cell of a hex one; [Relief.GroundAt] is the ground at any point,
// drawn between the corners as the terrain's mesh draws it, [Relief.Altitude] a cell's mean,
// [Relief.SetHeights] seeds it from a function ([MeanOfCells] from one of the cells), and it is the
// board's heights (board.Heights: At, Step, Top). A board that wraps folds its corners across the
// seam. [Relief.Climb] prices a step from cell to cell by how steep it is as [Climbing] says
// ([DefaultClimbing]: up costly, down gently, steep down again; free for the domains that fly);
// [Relief.SlopeAt] is the slope at a point along a way.
//
// Shaping ([Shaper], [NewShaper]) raises and lowers the ground ([Raise], [Lower]) a [Shaping] step
// at a time and levels it ([Level]), the ground round about following within the step between
// corners, as in Transport Tycoon; its System carries them out as they come and its Queues take
// them.
//
// The heights live on entities of their own, a run of [HeightsRun] corners each ([Heights]),
// written anew whenever the relief changes and taken back from a loaded game
// ([Relief.HeightsSystem]); [Relief.AltitudeSystem] keeps every mover at the ground under it plus
// its lift, a flyer flown by hand at its own height, all under their ceilings. The topography
// (plugins/topography) puts them to work.
package relief
