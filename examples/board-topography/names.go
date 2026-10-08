package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	IceCell = "ice"

	// effects
	BloodMoonEf = "blood moon"

	// roles
	MortalRole = "mortal"
	LunarRole  = "lunar"

	// commands
	BleedCmd = "bleed"

	// kinds of units
	UnitKind    = "unit"
	RivalKind   = "rival"
	HawkKind    = "hawk"
	PlateauKind = "plateau"

	// the stage and its scenes
	BoardTopographyStage = "board-topography"
	MainScene            = "main"
)

// snowyCell is the name of the kind of cell a kind turns into under snow.
func snowyCell(kind string) string { return "snowy " + kind }
