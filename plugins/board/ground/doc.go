// Package ground is what a board's ground is to the other plugins: the height of it at a point
// ([Heights], a topography's relief; none on a flat map) and what stands on it and holds sight
// back ([Cover], walked along a ray; [Readied] to read it all at once, for whoever walks it from
// several goroutines). Sight and navigation read them (board.Plugin.Heights, Cover); the board is
// collision's solid ground too (board.Plugin.WithCollision). [MeanOfCells] builds the heights out
// of one per cell.
//
// Where a road may lie is Transport Tycoon's rule, the board's loader's and the topography's
// shaping's alike: [RoadFits] a square cell's corners for the ways the road runs out by — level,
// an even ramp of [RoadSlope] at most run straight up, or a level diagonal it runs along with a
// third corner beside it; a bend, a fork or an end level — and [RoadJoins] two hexes' levels, only
// one.
package ground
