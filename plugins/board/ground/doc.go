// Package ground is what a board's ground is to the other plugins: the height of it at a point
// ([Heights]; a [Profile] knows it along a ray exactly), what stands on it and holds sight back
// ([Cover], walked along a ray) and where whoever moves over it stands ([Surfaces]: the ground or
// a deck, and the deck over it). [MeanOfCells] builds the heights out of one per cell; a [Shaper]
// lays a cell flat. Where a road may lie is Transport Tycoon's rule, [RoadFits] on a cell's
// corners on either grid: level, or an even ramp of [RoadSlope] at most straight up across
// opposite edges.
package ground
