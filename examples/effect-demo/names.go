package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	GrassCell = "grass"
	WaterCell = "water"

	// effects
	FrozenEf = "frozen"
	FrostEf  = "frost"
	IcedEf   = "iced"
	SlipEf   = "slip"

	// roles
	LakeRole   = "lake"
	WitchRole  = "witch"
	MortalRole = "mortal"

	// commands
	FreezeCmd = "freeze"

	// kinds of units
	WitchKind  = "witch"
	WalkerKind = "walker"
	BoatKind   = "boat"

	// the stage and its scenes
	EffectStage = "effect-demo"
	MainScene   = "main"
)
