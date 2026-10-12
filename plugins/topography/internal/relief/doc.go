// Package relief is the ground's heights under a board: how high every corner stands, the
// ground between them, what climbing it costs, how it is shaped, and the heights saved with the
// game.
//
// A [Relief] ([New]) holds a height at every corner of a square grid's lattice ([Corners]) or at
// every cell of a hex one; [Relief.GroundAt] is the ground at any point as it is drawn,
// [Relief.Altitude] a cell's mean, [Relief.SetHeights] seeds it from a function, and it is the
// board's heights (ground.Heights). [Relief.Climb] prices a step from cell to cell by how steep
// it is, [Relief.SlopeAt] is the slope at a point along a way.
//
// [Relief.Lift] raises and lowers the ground at a point, [Relief.Flatten] levels it between two
// and [Relief.Level] lays a cell flat, the ground round about following within a step between
// corners; [Relief.KeepRoads] takes a shaping back where it would break a road.
//
// The heights live on entities of their own, a run of [HeightsRun] each ([Heights]), kept by
// [Relief.HeightsSystem]; [Relief.AltitudeSystem] keeps every mover at the ground under it plus
// its lift. A game reads and sets the heights through its topography.Relief.
package relief
