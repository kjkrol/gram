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
	WinterEf = "winter" // the witch on the ice: the game waits for the player to decide
	CalmEf   = "calm"   // the witch answered: no winter asked again for a while
	SpringEf = "spring" // the lake thawed: the witch freezes nothing for a while

	// roles
	LakeRole   = "lake"
	WitchRole  = "witch"
	MortalRole = "mortal"

	// commands
	FreezeCmd    = "freeze"
	EndWinterCmd = "end winter"
	CalmWitchCmd = "calm the witch"
	ThawCmd      = "thaw the lake"
	SpringCmd    = "bring spring"

	// names and groups
	WitchName = "the witch"
	LakeGroup = "the lake"

	// scenes, and the elements of their screens
	MainScene      = "main"
	DecisionWindow = "decision"

	// kinds of units
	WitchKind  = "witch"
	WalkerKind = "walker"
	BoatKind   = "boat"
)
