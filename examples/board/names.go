package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	IceCell = "ice"

	// roles
	MortalRole = "mortal"

	// kinds of units
	UnitKind = "unit"
)

// snowyCell is the name of the kind of cell a kind turns into under snow.
func snowyCell(kind string) string { return "snowy " + kind }
