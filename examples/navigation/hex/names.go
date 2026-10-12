package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	GrassCell = "grass"
	WaterCell = "water"
	RoadCell  = "road"

	// kinds of units and of what stands on the board
	RedKind    = "red"
	BlueKind   = "blue"
	BridgeKind = "bridge"

	// sides
	PlayerSide = "player"

	// the stage and its scenes
	BoardNavigationHexStage = "board-navigation-hex-demo"
	MainScene               = "main"
)
