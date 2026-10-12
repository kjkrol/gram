// Package ground is what a board's ground is to the other plugins: the height of it at a point
// ([Heights], a topography's relief; none on a flat map) and what stands on it and holds sight
// back ([Cover], walked along a ray; [Readied] to read it all at once, for whoever walks it from
// several goroutines). Sight and navigation read them (board.Plugin.Heights, Cover); the board is
// collision's solid ground too (board.Plugin.WithCollision). [MeanOfCells] builds the heights out
// of one per cell. Heights that are a [Profile] know the ground along a ray exactly — the points
// where its slope changes — and sight reads them there in place of a sample every Step.
//
// Where a road may lie is Transport Tycoon's rule, the board's loader's and the topography's
// shaping's alike: [RoadFits] a square cell's corners for the ways the road runs out by — level,
// or an even ramp of [RoadSlope] at most run straight up; a diagonal, a bend, a fork or an end
// level — and [RoadJoins] two hexes' levels, only one.
//
// [Surfaces] is where whoever moves over the board stands: the ground, or a deck within a step of
// its feet, and the underside of a deck over it — a topography's relief, which knows the board's
// floors.
package ground
