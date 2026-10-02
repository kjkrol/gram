// Package relief is the ground's heights under a board: how high every corner stands, the ground
// between them, what climbing it costs, how it is shaped, and the heights saved with the game.
//
// A [Relief] ([New]) holds a height at every corner of a square grid's lattice — a cell's four
// corners ([Corners]) — or at every cell of a hex one; [Relief.GroundAt] is the ground at any point,
// drawn between the corners as the terrain's mesh draws it, [Relief.Altitude] a cell's mean,
// [Relief.SetHeights] seeds it from a function (relief.MeanOfCells from one of the cells), and it is the
// board's heights (ground.Heights: At, Step, Top). A board that wraps folds its corners across the
// seam. [Relief.Climb] prices a step from cell to cell by how steep it is as a relief.Climbing says
// (the public package's: up costly, down gently, steep down again; free for the domains that fly);
// [Relief.SlopeAt] is the slope at a point along a way.
//
// [Relief.Lift] raises and lowers the ground at a point and [Relief.Flatten] levels it between two,
// the ground round about following within a step between corners, as in Transport Tycoon: the
// topography's Raise, Lower and Level commands.
//
// The heights live on entities of their own, a run of [HeightsRun] corners each ([Heights]),
// written anew whenever the relief changes and taken back from a loaded game
// ([Relief.HeightsSystem]); [Relief.AltitudeSystem] keeps every mover at the ground under it plus
// its lift, a flyer flown by hand at its own height, all under their ceilings. The topography puts
// them to work; a game reads and sets the heights through its topography.Relief.
package relief
