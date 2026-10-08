package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	MeadowCell = "meadow"

	// kinds of units
	TravellerKind = "traveller"
	HostKind      = "host"

	// roles
	TravellerRole = "traveller"
	HostRole      = "host"

	// the nodes the hosts' conversations begin at (dialogs/*.yaml)
	MillerGreetNode = "miller.greet"
	SmithGreetNode  = "smith.greet"

	// the stage and its scenes
	DialogStage = "dialog-demo"
	MainScene   = "main"
)
