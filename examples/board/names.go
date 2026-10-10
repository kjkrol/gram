package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	WaterCell  = "water"
	EarthCell  = "earth"
	SandCell   = "sand"
	RockCell   = "rock"
	ForestCell = "forest"
	BrookCell  = "brook"
	StreamCell = "stream"
	RiverCell  = "river"
	FordCell   = "ford"
	RoadCell   = "road"
	BridgeCell = "bridge"
	IceCell    = "ice"

	// roles
	MortalRole = "mortal"

	// kinds of units
	UnitKind = "unit"

	// sides
	PlayerSide = "player"

	// the stage and its scenes
	BoardStage = "board"
	MainScene  = "main"
)

// snowyCell is the name of the kind of cell a kind turns into under snow.
func snowyCell(kind string) string { return "snowy " + kind }
