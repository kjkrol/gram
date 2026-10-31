// Package relief is the ground's heights under a board: how high every corner stands, the
// ground between them, how steep a step is, how it is shaped, and the heights saved with the
// game.
//
// A [Relief] ([New]) holds a height at every corner of a board's cells, the neighbouring cells
// sharing it: a square grid's lattice ([Corners]) or a hex grid's corners ([Relief.Hex], laid as
// [HexCorners] says). [Relief.GroundAt] is the ground at any point as it is drawn, two triangles a
// square cell and six a hex one, from its middle at the mean of its corners out to them;
// [Relief.Along] is the same along a ray, exactly. [Relief.Altitude] is a cell's mean,
// [Relief.SetHeights] seeds the corners from a function, and the relief is the board's heights
// (ground.Heights). [Relief.Rise] is how steep a step from cell to cell is, the rise between
// their middles over the run.
//
// [Relief.Lift] raises and lowers the corner nearest a point, [Relief.Flatten] levels the corners
// between two points and [Relief.Level] lays a cell flat, the ground round about following within
// a step between corners along the cells' edges; [Relief.KeepRoads] takes a shaping back where it
// would break a road.
//
// The heights live on entities of their own, a run of [HeightsRun] each ([Heights]), kept by
// [Relief.HeightsSystem]; a save holding a hex grid's level a cell loads with every corner at the
// mean of the cells meeting there. [Relief.AltitudeSystem] keeps every mover at the ground under
// it plus its lift. A game reads and sets the heights through its topography.Relief.
package relief
