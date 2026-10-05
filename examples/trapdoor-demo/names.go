package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	GrassCell  = "grass"
	BoardsCell = "boards"
	PitCell    = "pit"

	// effects
	OpenEf  = "open"
	HasteEf = "haste"

	// roles
	MortalRole = "mortal"

	// commands
	HastenCmd = "hasten"

	// kinds of units
	ScoutKind    = "scout"
	WandererKind = "wanderer"
)

// pullCmd is the name of the command pulling the lever called lever.
func pullCmd(lever string) string { return "pull " + lever }
