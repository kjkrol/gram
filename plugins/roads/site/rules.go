package site

// Rules are what the game allows a road: zero allows nothing, so the zero Rules lay a road on
// level ground alone, crossing nothing, with no earth moved.
type Rules struct {
	// Cut and Fill are the most a corner may be dug down and raised, in world units.
	Cut, Fill float32
	// Earth is the most earth one plan may move, cut and fill together, in world units cubed.
	Earth float32
	// Spread is how far from the road's cells, in cells, the levelling may ripple as the ground
	// round a moved corner follows it; 0 keeps it to the road's own cells.
	Spread int
	// Slope is the steepest ramp a Graded road may lie on, in radians, ground.RoadSlope at most;
	// for a track, the steepest ground it may run over.
	Slope float64
	// Crossings lets a road cross and join the roads standing.
	Crossings bool
	// Decks lets a road run over a bridge's deck, Fords through a ford.
	Decks, Fords bool
	// Longest is the most cells one plan may lay, 0 or less no limit: a cap, not an allowance,
	// so the one zero that forbids nothing.
	Longest int
	// Ends lets a road end loose in a cell; without it a road ends on a road, a deck or its
	// target.
	Ends bool
	// Metre weighs a step's length in Route's cost (0 counts a step of the grid's Distance as 1)
	// and Climb how much dearer a metre is per unit of the slope left after the permitted
	// levelling; Cubic weighs the earth a leap's heads move, where a map maker's bridges lead.
	Metre, Cubic, Climb float32
}
