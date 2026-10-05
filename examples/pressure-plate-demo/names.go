package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	GrassCell  = "grass"
	BoardsCell = "boards"
	PlateCell  = "plate"
	PitCell    = "pit"

	// effects
	OpenEf = "open"

	// roles
	PlateRole  = "plate"
	MortalRole = "mortal"

	// kinds of units
	ScoutKind    = "scout"
	WandererKind = "wanderer"
)

// openCmd is the name of the command opening the strip of trapdoors called group.
func openCmd(group string) string { return "open " + group }
