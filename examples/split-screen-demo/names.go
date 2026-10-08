package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	FloorCell = "floor"
	WallCell  = "wall"

	// kinds of units
	RedKind  = "red"
	BlueKind = "blue"

	// the blocks, by name: what each player's camera follows
	RedBlock  = "red block"
	BlueBlock = "blue block"

	// the stage and its scenes
	SplitScreenStage = "split-screen-demo"
	MainScene        = "main"
)
