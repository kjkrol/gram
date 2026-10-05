package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	GrassCell  = "grass"
	WallCell   = "wall"
	ForestCell = "forest"
	RoadCell   = "road"
	HillCell   = "hill"

	// kinds of units
	RedKind    = "red"
	BlueKind   = "blue"
	YellowKind = "yellow"
	HawkKind   = "hawk"
)

// scouts are the kinds of walkers, in the order of their colours.
var scouts = []string{RedKind, BlueKind, YellowKind}
