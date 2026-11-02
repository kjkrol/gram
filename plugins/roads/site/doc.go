// Package site is where a road may run and what laying it takes: [Route] finds the cheapest way
// from cell to cell over a [Ground] under the [Rules], [Plan] turns a way into an [Estimate]
// (the cells, their links, the corners levelled and what it all comes to) or says why not
// ([Refusal]). It runs no game, so the roads plugin and cmd/gramap keep one set of rules.
//
// A road is a cell.Way laid cell after cell. A Graded one lies as Transport Tycoon lays it
// (ground.RoadFits: level, or an even ramp straight up across opposite edges), a track follows
// the ground as it lies. The Rules are what the game allows, zero allowing nothing: how far a
// corner may be cut or filled and how much earth moved in all, how far from the road the
// levelling may ripple, how steep a ramp may be, whether a road may cross another, run over a
// bridge's deck or through a ford, how long one plan may be and whether it may end loose. Route
// prices a step by the slope the ground would have after the permitted levelling, so a road
// goes where the game would let it be built, and refuses a step the levelling cannot make fit;
// the levelling itself and its earth go into the Estimate. A [Planner] keeps Route's search for
// one road over one Ground, reads the ground anew on every route and allocates nothing once
// warm; [Leaps] add a map maker's bridges to the ways it may take.
//
// Plan levels the ground as cmd/gramap graded Madeira's roads, on a square and a hex grid alike:
// the corners the neighbouring cells share classed together where a road cell lies level and
// along the edges where it ramps, the ramps relaxed to the slope allowed, the ground round the
// road following within the shaping's step, the corners that may not move holding. The
// Estimate's corners are what grading the cells of its Ways in order leaves on the ground;
// [Estimate.Levelled] is one cell's corners as the plan leaves them.
package site
