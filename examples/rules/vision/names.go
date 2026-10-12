package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// effects
	FleeingEf = "fleeing"
	LookedEf  = "looked"

	// roles
	PreyRole     = "prey"
	PredatorRole = "predator"
	SkittishRole = "skittish"

	// commands
	FleeCmd = "flee"

	// kinds of units
	PreyKind   = "prey"
	HunterKind = "hunter"

	// the stage and its scenes
	VisionStage = "vision-demo"
	MainScene   = "main"
)
